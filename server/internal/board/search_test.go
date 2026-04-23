package board

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oceanplexian/lwts/server/internal/repo"
)

func TestSearch_LikeFallbackWithoutEmbed(t *testing.T) {
	ds, users, boards, cards, _ := setupTest(t)
	user := createTestUser(t, users)

	board, err := boards.Create(context.Background(), "Test Board", "TST", user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cards.Create(context.Background(), board.ID, repo.CardCreate{
		ColumnID: "todo", Title: "Login bug needs fix",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := cards.Create(context.Background(), board.ID, repo.CardCreate{
		ColumnID: "todo", Title: "Logout works fine",
	}); err != nil {
		t.Fatal(err)
	}

	h := NewSearchHandler(ds)
	req := httptest.NewRequest("GET", "/api/v1/search?q=login", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}
	var results []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &results)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	if results[0]["title"] != "Login bug needs fix" {
		t.Fatalf("wrong card: %v", results[0]["title"])
	}
	// New fields must be populated even on the lexical path so agents see a
	// consistent shape across search modes.
	if results[0]["match_kind"] != "lexical" {
		t.Errorf("match_kind: %v", results[0]["match_kind"])
	}
	if results[0]["score"] == nil {
		t.Errorf("score missing")
	}
	// Response headers convey mode + total count.
	if got := w.Header().Get("X-Search-Mode"); got != "lexical" {
		t.Errorf("X-Search-Mode: %q", got)
	}
	if got := w.Header().Get("X-Total-Matches"); got != "1" {
		t.Errorf("X-Total-Matches: %q", got)
	}
}

func TestSearch_RequiresAtLeastOneFilter(t *testing.T) {
	ds, _, _, _, _ := setupTest(t)
	h := NewSearchHandler(ds)

	req := httptest.NewRequest("GET", "/api/v1/search", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSearch_ExcludeDoneWhenRequested(t *testing.T) {
	ds, users, boards, cards, _ := setupTest(t)
	user := createTestUser(t, users)
	board, _ := boards.Create(context.Background(), "Test Board", "TST", user.ID)
	_, _ = cards.Create(context.Background(), board.ID, repo.CardCreate{
		ColumnID: "todo", Title: "refactor login flow",
	})
	_, _ = cards.Create(context.Background(), board.ID, repo.CardCreate{
		ColumnID: "done", Title: "login bug fixed",
	})

	h := NewSearchHandler(ds)
	req := httptest.NewRequest("GET", "/api/v1/search?q=login&include_done=false", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	h.Search(w, req)

	var results []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &results)
	if len(results) != 1 {
		t.Fatalf("expected 1 (done excluded), got %d: %v", len(results), results)
	}
	if results[0]["title"] != "refactor login flow" {
		t.Fatalf("wrong card: %v", results[0]["title"])
	}
}

func TestSearch_IncludesDoneByDefault(t *testing.T) {
	// Backward-compat: the web UI relies on the default including everything.
	ds, users, boards, cards, _ := setupTest(t)
	user := createTestUser(t, users)
	board, _ := boards.Create(context.Background(), "Test Board", "TST", user.ID)
	_, _ = cards.Create(context.Background(), board.ID, repo.CardCreate{
		ColumnID: "todo", Title: "pending login fix",
	})
	_, _ = cards.Create(context.Background(), board.ID, repo.CardCreate{
		ColumnID: "done", Title: "login bug already fixed",
	})

	h := NewSearchHandler(ds)
	req := httptest.NewRequest("GET", "/api/v1/search?q=login", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	h.Search(w, req)

	var results []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &results)
	if len(results) != 2 {
		t.Fatalf("expected 2 (default includes done), got %d", len(results))
	}
}

func TestSearch_MinScoreFiltersOut(t *testing.T) {
	ds, users, boards, cards, _ := setupTest(t)
	user := createTestUser(t, users)
	board, _ := boards.Create(context.Background(), "Test Board", "TST", user.ID)
	// Title match -> score 0.9; description-only -> score 0.6
	_, _ = cards.Create(context.Background(), board.ID, repo.CardCreate{
		ColumnID: "todo", Title: "login works",
	})
	_, _ = cards.Create(context.Background(), board.ID, repo.CardCreate{
		ColumnID: "todo", Title: "unrelated", Description: "login somewhere",
	})

	h := NewSearchHandler(ds)
	req := httptest.NewRequest("GET", "/api/v1/search?q=login&min_score=0.8", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	h.Search(w, req)

	var results []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &results)
	if len(results) != 1 {
		t.Fatalf("expected 1 high-score result, got %d: %v", len(results), results)
	}
	if results[0]["title"] != "login works" {
		t.Fatalf("wrong card: %v", results[0]["title"])
	}
}

func TestSearch_IncludesSnippet(t *testing.T) {
	ds, users, boards, cards, _ := setupTest(t)
	user := createTestUser(t, users)
	board, _ := boards.Create(context.Background(), "Test Board", "TST", user.ID)
	_, _ = cards.Create(context.Background(), board.ID, repo.CardCreate{
		ColumnID:    "todo",
		Title:       "Fix scroll bug",
		Description: "The header does not stick when the page scrolls. Users are frustrated.",
	})

	h := NewSearchHandler(ds)
	req := httptest.NewRequest("GET", "/api/v1/search?q=scroll", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	h.Search(w, req)

	var results []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &results)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	snippet, _ := results[0]["snippet"].(string)
	if snippet == "" {
		t.Fatal("expected non-empty snippet")
	}
}

func TestSearch_TicketKeyPinsExactMatchFirst(t *testing.T) {
	// Typing the literal ticket key into the search box should always return
	// that exact card first, regardless of whatever else also LIKE-matches.
	ds, users, boards, cards, _ := setupTest(t)
	user := createTestUser(t, users)
	b, _ := boards.Create(context.Background(), "Test Board", "TST", user.ID)
	// Decoy whose title mentions "TST-3" so it also LIKE-matches the query.
	_, _ = cards.Create(context.Background(), b.ID, repo.CardCreate{
		ColumnID: "todo", Title: "see TST-3 for context",
	})
	_, _ = cards.Create(context.Background(), b.ID, repo.CardCreate{
		ColumnID: "todo", Title: "depends on TST-3",
	})
	// Third card auto-keys to TST-3 — that's what we search for.
	target, _ := cards.Create(context.Background(), b.ID, repo.CardCreate{
		ColumnID: "todo", Title: "the actual ticket",
	})
	if target.Key != "TST-3" {
		t.Fatalf("expected target key TST-3, got %q", target.Key)
	}

	h := NewSearchHandler(ds)
	// Lowercase to confirm case-insensitive matching.
	req := httptest.NewRequest("GET", "/api/v1/search?q=tst-3", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}
	var results []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &results)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0]["id"] != target.ID {
		t.Fatalf("expected pinned card %q first, got %q (title=%v)",
			target.ID, results[0]["id"], results[0]["title"])
	}
	if results[0]["match_kind"] != "key" {
		t.Errorf("expected match_kind=key, got %v", results[0]["match_kind"])
	}
	if score, _ := results[0]["score"].(float64); score < 0.99 {
		t.Errorf("expected score 1.0 for key pin, got %v", score)
	}
}

func TestSearch_PartialKeyMatchesViaLike(t *testing.T) {
	// "TST-" alone isn't a complete key but should still surface every card
	// whose key starts with that prefix via the LIKE clause on c.key.
	ds, users, boards, cards, _ := setupTest(t)
	user := createTestUser(t, users)
	b, _ := boards.Create(context.Background(), "Test Board", "TST", user.ID)
	_, _ = cards.Create(context.Background(), b.ID, repo.CardCreate{
		ColumnID: "todo", Title: "alpha",
	})
	_, _ = cards.Create(context.Background(), b.ID, repo.CardCreate{
		ColumnID: "todo", Title: "beta",
	})

	h := NewSearchHandler(ds)
	req := httptest.NewRequest("GET", "/api/v1/search?q=TST-", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	h.Search(w, req)

	var results []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &results)
	if len(results) != 2 {
		t.Fatalf("expected 2 cards via key prefix, got %d: %v", len(results), results)
	}
}

func TestSearch_KeyShapeMissesFallsBackToLike(t *testing.T) {
	// A key-shaped query that doesn't match any card key shouldn't pin
	// anything spurious; the lexical fallback should still run cleanly.
	ds, users, boards, cards, _ := setupTest(t)
	user := createTestUser(t, users)
	b, _ := boards.Create(context.Background(), "Test Board", "TST", user.ID)
	_, _ = cards.Create(context.Background(), b.ID, repo.CardCreate{
		ColumnID: "todo", Title: "unrelated work",
	})

	h := NewSearchHandler(ds)
	req := httptest.NewRequest("GET", "/api/v1/search?q=ZZZ-999", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var results []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &results)
	if len(results) != 0 {
		t.Fatalf("expected 0 results for non-existent key, got %d: %v", len(results), results)
	}
}

func TestReadSearchMode_DefaultsToLexical(t *testing.T) {
	ds, _, _, _, _ := setupTest(t)
	req := httptest.NewRequest("GET", "/api/v1/search?q=x", nil)
	if mode := readSearchMode(req, ds); mode != "lexical" {
		t.Fatalf("default mode: %q", mode)
	}
}

func TestReadSearchMode_SemanticFromSettings(t *testing.T) {
	ds, _, _, _, _ := setupTest(t)
	_, err := ds.Exec(context.Background(),
		`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, datetime('now'))`,
		"general", `{"search_mode":"semantic"}`)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/api/v1/search?q=x", nil)
	if mode := readSearchMode(req, ds); mode != "semantic" {
		t.Fatalf("expected semantic, got %q", mode)
	}
}
