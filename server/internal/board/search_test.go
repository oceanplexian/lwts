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
	// Don't call SetEmbed — the dispatcher should fall through to LIKE.

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
