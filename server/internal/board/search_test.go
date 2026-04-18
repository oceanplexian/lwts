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
