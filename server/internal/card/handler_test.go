package card

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oceanplexian/lwts/server/internal/auth"
	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/oceanplexian/lwts/server/internal/repo"
	"github.com/oceanplexian/lwts/server/migrations"
)

func setupTest(t *testing.T) (*repo.UserRepository, *repo.BoardRepository, *repo.CardRepository, *repo.CommentRepository) {
	_, users, boards, cards, comments := setupTestWithDS(t)
	return users, boards, cards, comments
}

func setupTestWithDS(t *testing.T) (db.Datasource, *repo.UserRepository, *repo.BoardRepository, *repo.CardRepository, *repo.CommentRepository) {
	t.Helper()
	ds, err := db.NewSQLiteDatasource("sqlite://:memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ds.Close() })

	if err := db.Migrate(context.Background(), ds, migrations.FS); err != nil {
		t.Fatal(err)
	}

	return ds,
		repo.NewUserRepository(ds),
		repo.NewBoardRepository(ds),
		repo.NewCardRepository(ds),
		repo.NewCommentRepository(ds)
}

func withUser(r *http.Request, u repo.User) *http.Request {
	ctx := context.WithValue(r.Context(), auth.UserContextKey, &u)
	return r.WithContext(ctx)
}

func noopAuth(next http.Handler) http.Handler { return next }

func TestCardCreate(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)

	body, _ := json.Marshal(createCardReq{Title: "Fix bug", ColumnID: "todo"})

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/boards/{boardId}/cards", noopAuth(http.HandlerFunc(h.Create)))

	req := httptest.NewRequest("POST", "/api/v1/boards/"+board.ID+"/cards", bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}

	var card repo.Card
	_ = json.Unmarshal(w.Body.Bytes(), &card)
	if card.Key != "LWTS-1" {
		t.Errorf("key: %s", card.Key)
	}
	if card.Title != "Fix bug" {
		t.Errorf("title: %s", card.Title)
	}
	if card.CustomFields == nil || len(card.CustomFields) != 0 {
		t.Errorf("custom_fields = %#v, want empty object", card.CustomFields)
	}
}

func setBoardCustomFields(t *testing.T, boards *repo.BoardRepository, board repo.Board, defs []repo.CustomFieldDefinition) repo.Board {
	t.Helper()
	settingsBytes, _ := json.Marshal(map[string]any{"custom_fields": defs})
	settings, _, err := repo.NormalizeBoardSettings(string(settingsBytes))
	if err != nil {
		t.Fatalf("normalize settings: %v", err)
	}
	updated, err := boards.Update(context.Background(), board.ID, repo.BoardUpdate{Settings: &settings})
	if err != nil {
		t.Fatalf("update board settings: %v", err)
	}
	return updated
}

func TestCardCreateWithCustomFields(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)
	setBoardCustomFields(t, boards, board, []repo.CustomFieldDefinition{
		{ID: "customer", Name: "Customer", Type: repo.CustomFieldTypeText, Required: true},
		{ID: "severity", Name: "Severity", Type: repo.CustomFieldTypeSelect, Options: []repo.CustomFieldOption{
			{ID: "sev1", Label: "SEV 1"},
			{ID: "sev2", Label: "SEV 2"},
		}},
	})

	body, _ := json.Marshal(createCardReq{
		Title:        "Fix bug",
		ColumnID:     "todo",
		CustomFields: map[string]any{"customer": "Acme", "severity": "sev1"},
	})

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/boards/{boardId}/cards", noopAuth(http.HandlerFunc(h.Create)))

	req := httptest.NewRequest("POST", "/api/v1/boards/"+board.ID+"/cards", bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}
	var card repo.Card
	_ = json.Unmarshal(w.Body.Bytes(), &card)
	if card.CustomFields["customer"] != "Acme" || card.CustomFields["severity"] != "sev1" {
		t.Fatalf("custom fields = %#v", card.CustomFields)
	}
}

func TestCardCreateRejectsInvalidCustomFields(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)
	setBoardCustomFields(t, boards, board, []repo.CustomFieldDefinition{
		{ID: "customer", Name: "Customer", Type: repo.CustomFieldTypeText, Required: true},
		{ID: "severity", Name: "Severity", Type: repo.CustomFieldTypeSelect, Options: []repo.CustomFieldOption{{ID: "sev1", Label: "SEV 1"}}},
	})

	cases := []createCardReq{
		{Title: "Missing required", ColumnID: "todo", CustomFields: map[string]any{"severity": "sev1"}},
		{Title: "Bad option", ColumnID: "todo", CustomFields: map[string]any{"customer": "Acme", "severity": "sev2"}},
		{Title: "Unknown", ColumnID: "todo", CustomFields: map[string]any{"customer": "Acme", "unknown": "x"}},
	}

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/boards/{boardId}/cards", noopAuth(http.HandlerFunc(h.Create)))
	for _, tc := range cases {
		body, _ := json.Marshal(tc)
		req := httptest.NewRequest("POST", "/api/v1/boards/"+board.ID+"/cards", bytes.NewReader(body))
		req = withUser(req, user)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d, body: %s", tc.Title, w.Code, w.Body.String())
		}
	}
}

func TestCardCreateMissingTitle(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)

	body, _ := json.Marshal(createCardReq{ColumnID: "todo"})

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/boards/{boardId}/cards", noopAuth(http.HandlerFunc(h.Create)))

	req := httptest.NewRequest("POST", "/api/v1/boards/"+board.ID+"/cards", bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCardUpdateConflict(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)
	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "todo", Title: "Card"})

	// Update with wrong version
	title := "Updated"
	body, _ := json.Marshal(updateCardReq{Title: &title, Version: 99})

	mux := http.NewServeMux()
	mux.Handle("PUT /api/v1/cards/{id}", noopAuth(http.HandlerFunc(h.Update)))

	req := httptest.NewRequest("PUT", "/api/v1/cards/"+card.ID, bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCardUpdateSuccess(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)
	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "todo", Title: "Card"})

	title := "Updated"
	body, _ := json.Marshal(updateCardReq{Title: &title, Version: card.Version})

	mux := http.NewServeMux()
	mux.Handle("PUT /api/v1/cards/{id}", noopAuth(http.HandlerFunc(h.Update)))

	req := httptest.NewRequest("PUT", "/api/v1/cards/"+card.ID, bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}

	var updated repo.Card
	_ = json.Unmarshal(w.Body.Bytes(), &updated)
	if updated.Title != "Updated" {
		t.Errorf("title: %s", updated.Title)
	}
	if updated.Version != 2 {
		t.Errorf("version: %d", updated.Version)
	}
}

func TestCardUpdateCustomFields(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)
	setBoardCustomFields(t, boards, board, []repo.CustomFieldDefinition{
		{ID: "customer", Name: "Customer", Type: repo.CustomFieldTypeText},
		{ID: "severity", Name: "Severity", Type: repo.CustomFieldTypeSelect, Options: []repo.CustomFieldOption{
			{ID: "sev1", Label: "SEV 1"},
			{ID: "sev2", Label: "SEV 2"},
		}},
	})
	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{
		ColumnID:     "todo",
		Title:        "Card",
		CustomFields: map[string]string{"customer": "Acme", "severity": "sev1"},
	})

	body, _ := json.Marshal(updateCardReq{
		CustomFields: &map[string]any{"customer": "", "severity": "sev2"},
		Version:      card.Version,
	})

	mux := http.NewServeMux()
	mux.Handle("PUT /api/v1/cards/{id}", noopAuth(http.HandlerFunc(h.Update)))
	req := httptest.NewRequest("PUT", "/api/v1/cards/"+card.ID, bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}
	var updated repo.Card
	_ = json.Unmarshal(w.Body.Bytes(), &updated)
	if _, ok := updated.CustomFields["customer"]; ok {
		t.Fatalf("customer should be cleared: %#v", updated.CustomFields)
	}
	if updated.CustomFields["severity"] != "sev2" {
		t.Fatalf("severity = %#v", updated.CustomFields)
	}
}

func TestCardMove(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)
	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "todo", Title: "Card"})

	body, _ := json.Marshal(moveCardReq{ColumnID: "done", Position: 0, Version: card.Version})

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/cards/{id}/move", noopAuth(http.HandlerFunc(h.Move)))

	req := httptest.NewRequest("POST", "/api/v1/cards/"+card.ID+"/move", bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}

	var moved repo.Card
	_ = json.Unmarshal(w.Body.Bytes(), &moved)
	if moved.ColumnID != "done" {
		t.Errorf("column: %s", moved.ColumnID)
	}
}

func TestCardDelete(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)
	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "todo", Title: "Card"})

	mux := http.NewServeMux()
	mux.Handle("DELETE /api/v1/cards/{id}", noopAuth(http.HandlerFunc(h.Delete)))

	req := httptest.NewRequest("DELETE", "/api/v1/cards/"+card.ID, nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	// Verify it's gone
	req = httptest.NewRequest("GET", "/api/v1/cards/"+card.ID, nil)
	req = withUser(req, user)
	w = httptest.NewRecorder()

	mux2 := http.NewServeMux()
	mux2.Handle("GET /api/v1/cards/{id}", noopAuth(http.HandlerFunc(h.Get)))
	mux2.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", w.Code)
	}
}

func TestCardGetWithComments(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)
	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "todo", Title: "Card"})
	_, _ = comments.Create(ctx, card.ID, user.ID, "Comment 1")
	_, _ = comments.Create(ctx, card.ID, user.ID, "Comment 2")

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/cards/{id}", noopAuth(http.HandlerFunc(h.Get)))

	req := httptest.NewRequest("GET", "/api/v1/cards/"+card.ID, nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}

	var resp map[string]json.RawMessage
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	var cmts []repo.Comment
	_ = json.Unmarshal(resp["comments"], &cmts)
	if len(cmts) != 2 {
		t.Errorf("comments: %d", len(cmts))
	}
}

func TestCardGetByKey(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)
	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "todo", Title: "Card"})

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/cards/{id}", noopAuth(http.HandlerFunc(h.Get)))

	// Hit the endpoint with the human-readable key (LWTS-1) instead of the UUID.
	req := httptest.NewRequest("GET", "/api/v1/cards/"+card.Key, nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]json.RawMessage
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	var got repo.Card
	_ = json.Unmarshal(resp["card"], &got)
	if got.ID != card.ID {
		t.Errorf("id: %s, want %s", got.ID, card.ID)
	}
	if got.Key != card.Key {
		t.Errorf("key: %s, want %s", got.Key, card.Key)
	}
}

func TestCardGetByKeyMissing(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	_, _ = boards.Create(ctx, "B", "LWTS", user.ID)

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/cards/{id}", noopAuth(http.HandlerFunc(h.Get)))

	// A well-formed key that doesn't exist should be 404, not 500.
	req := httptest.NewRequest("GET", "/api/v1/cards/LWTS-9999", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing key, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestClearDoneMovesAllDoneCards(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)

	// Create cards across columns; only "done"-type ones should move.
	_, _ = cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "todo", Title: "Keep me 1"})
	_, _ = cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "in-progress", Title: "Keep me 2"})
	for i := 0; i < 5; i++ {
		_, _ = cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "done", Title: "Done card"})
	}

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/boards/{boardId}/clear-done", noopAuth(http.HandlerFunc(h.ClearDone)))

	req := httptest.NewRequest("POST", "/api/v1/boards/"+board.ID+"/clear-done", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}

	var moved []repo.Card
	if err := json.Unmarshal(w.Body.Bytes(), &moved); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(moved) != 5 {
		t.Fatalf("expected 5 moved, got %d", len(moved))
	}
	for _, c := range moved {
		if c.ColumnID != "cleared" {
			t.Errorf("card %s column = %q, want cleared", c.ID, c.ColumnID)
		}
	}

	// The board should have zero cards left in the done column.
	all, _ := cards.ListByBoard(ctx, board.ID)
	for _, c := range all {
		if c.ColumnID == "done" {
			t.Errorf("card %s still in done after clear", c.ID)
		}
	}
}

func TestClearDoneEmptyBoard(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/boards/{boardId}/clear-done", noopAuth(http.HandlerFunc(h.ClearDone)))

	req := httptest.NewRequest("POST", "/api/v1/boards/"+board.ID+"/clear-done", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var moved []repo.Card
	_ = json.Unmarshal(w.Body.Bytes(), &moved)
	if len(moved) != 0 {
		t.Errorf("expected 0 moved on empty board, got %d", len(moved))
	}
}

func TestClearDoneHandlesLargeDoneCardSet(t *testing.T) {
	ds, users, boards, cards, comments := setupTestWithDS(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)

	_, err := ds.Exec(ctx, `INSERT INTO cards (id, board_id, column_id, title, key, position) VALUES ($1, $2, 'cleared', 'Already cleared', 'LWTS-0', 0)`, "cleared-0", board.ID)
	if err != nil {
		t.Fatalf("seed cleared card: %v", err)
	}
	_, err = ds.Exec(ctx, `INSERT INTO cards (id, board_id, column_id, title, key, position) VALUES ($1, $2, 'todo', 'Keep me', 'LWTS-keep', 0)`, "keep-0", board.ID)
	if err != nil {
		t.Fatalf("seed todo card: %v", err)
	}
	for i := 0; i < 1005; i++ {
		id := fmt.Sprintf("done-%04d", i)
		key := fmt.Sprintf("LWTS-%04d", i+1)
		if _, err := ds.Exec(ctx, `INSERT INTO cards (id, board_id, column_id, title, key, position) VALUES ($1, $2, 'done', $3, $4, $5)`, id, board.ID, "Done card", key, i); err != nil {
			t.Fatalf("seed done card %d: %v", i, err)
		}
	}

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/boards/{boardId}/clear-done", noopAuth(http.HandlerFunc(h.ClearDone)))

	req := httptest.NewRequest("POST", "/api/v1/boards/"+board.ID+"/clear-done", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}

	var moved []repo.Card
	if err := json.Unmarshal(w.Body.Bytes(), &moved); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(moved) != 1005 {
		t.Fatalf("expected 1005 moved, got %d", len(moved))
	}

	seenPositions := make(map[int]bool, len(moved))
	for _, c := range moved {
		if c.ColumnID != "cleared" {
			t.Fatalf("card %s column = %q, want cleared", c.ID, c.ColumnID)
		}
		if c.Position < 1 || c.Position > 1005 {
			t.Fatalf("card %s position = %d, want 1..1005", c.ID, c.Position)
		}
		seenPositions[c.Position] = true
	}
	if len(seenPositions) != 1005 {
		t.Fatalf("expected 1005 unique moved positions, got %d", len(seenPositions))
	}

	keep, err := cards.GetByID(ctx, "keep-0")
	if err != nil {
		t.Fatalf("get keep card: %v", err)
	}
	if keep.ColumnID != "todo" {
		t.Fatalf("keep card column = %q, want todo", keep.ColumnID)
	}
	all, _ := cards.ListByBoard(ctx, board.ID)
	for _, c := range all {
		if c.ColumnID == "done" {
			t.Fatalf("card %s still in done after clear", c.ID)
		}
	}
}

// ── Epic Tests ──

func TestCreateEpicCard(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)

	body, _ := json.Marshal(createCardReq{Title: "Platform Migration", Tag: "epic", ColumnID: "backlog"})
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/boards/{boardId}/cards", noopAuth(http.HandlerFunc(h.Create)))

	req := httptest.NewRequest("POST", "/api/v1/boards/"+board.ID+"/cards", bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}
	var epic repo.Card
	_ = json.Unmarshal(w.Body.Bytes(), &epic)
	if epic.Tag != "epic" {
		t.Errorf("tag: %s, want epic", epic.Tag)
	}
	if epic.EpicID != nil {
		t.Errorf("epic card should not have epic_id set")
	}
}

func TestCreateCardWithEpicID(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)
	epic, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "backlog", Title: "Epic", Tag: "epic"})

	body, _ := json.Marshal(createCardReq{Title: "Child Card", ColumnID: "backlog", EpicID: &epic.ID})
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/boards/{boardId}/cards", noopAuth(http.HandlerFunc(h.Create)))

	req := httptest.NewRequest("POST", "/api/v1/boards/"+board.ID+"/cards", bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}
	var child repo.Card
	_ = json.Unmarshal(w.Body.Bytes(), &child)
	if child.EpicID == nil || *child.EpicID != epic.ID {
		t.Errorf("epic_id: %v, want %s", child.EpicID, epic.ID)
	}
}

func TestMoveCardIntoEpic(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)
	epic, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "backlog", Title: "Epic", Tag: "epic"})
	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "backlog", Title: "Task"})

	epicID := epic.ID
	body, _ := json.Marshal(moveCardReq{ColumnID: "todo", Position: 0, Version: card.Version, EpicID: &epicID})

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/cards/{id}/move", noopAuth(http.HandlerFunc(h.Move)))

	req := httptest.NewRequest("POST", "/api/v1/cards/"+card.ID+"/move", bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}

	var moved repo.Card
	_ = json.Unmarshal(w.Body.Bytes(), &moved)
	if moved.ColumnID != "todo" {
		t.Errorf("column: %s, want todo", moved.ColumnID)
	}
	if moved.EpicID == nil || *moved.EpicID != epic.ID {
		t.Errorf("epic_id: %v, want %s", moved.EpicID, epic.ID)
	}
}

func TestMoveCardOutOfEpic(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)
	epic, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "backlog", Title: "Epic", Tag: "epic"})
	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "backlog", Title: "Task", EpicID: &epic.ID})

	// Move out: set epic_id to empty string (clear)
	emptyEpic := ""
	body, _ := json.Marshal(moveCardReq{ColumnID: "backlog", Position: 0, Version: card.Version, EpicID: &emptyEpic})

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/cards/{id}/move", noopAuth(http.HandlerFunc(h.Move)))

	req := httptest.NewRequest("POST", "/api/v1/cards/"+card.ID+"/move", bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}

	var moved repo.Card
	_ = json.Unmarshal(w.Body.Bytes(), &moved)
	if moved.EpicID != nil {
		t.Errorf("epic_id should be nil after removing from epic, got: %v", *moved.EpicID)
	}
}

func TestUpdateCardEpicID(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)
	epic, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "backlog", Title: "Epic", Tag: "epic"})
	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "backlog", Title: "Task"})

	// Assign to epic via update
	body, _ := json.Marshal(updateCardReq{EpicID: &epic.ID, Version: card.Version})
	mux := http.NewServeMux()
	mux.Handle("PUT /api/v1/cards/{id}", noopAuth(http.HandlerFunc(h.Update)))

	req := httptest.NewRequest("PUT", "/api/v1/cards/"+card.ID, bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}

	var updated repo.Card
	_ = json.Unmarshal(w.Body.Bytes(), &updated)
	if updated.EpicID == nil || *updated.EpicID != epic.ID {
		t.Errorf("epic_id: %v, want %s", updated.EpicID, epic.ID)
	}

	// Clear epic via update (empty string = clear)
	emptyEpic := ""
	body2, _ := json.Marshal(updateCardReq{EpicID: &emptyEpic, Version: updated.Version})
	req2 := httptest.NewRequest("PUT", "/api/v1/cards/"+card.ID, bytes.NewReader(body2))
	req2 = withUser(req2, user)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("clear epic status: %d, body: %s", w2.Code, w2.Body.String())
	}

	var cleared repo.Card
	_ = json.Unmarshal(w2.Body.Bytes(), &cleared)
	if cleared.EpicID != nil {
		t.Errorf("epic_id should be nil after clear, got: %v", *cleared.EpicID)
	}
}

// ── Transition Rules Tests ──

func TestTransitionBlockedToDone(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)

	// Enable no_blocked_to_done rule
	settings := `{"transition_rules":{"no_blocked_to_done":true}}`
	_, _ = boards.Update(ctx, board.ID, repo.BoardUpdate{Settings: &settings})

	// Create a card with blocked dependencies
	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "todo", Title: "Blocked Card"})
	blockedIDs := `["some-blocking-card-id"]`
	_, _ = cards.Update(ctx, card.ID, card.Version, repo.CardUpdate{BlockedCardIDs: &blockedIDs})
	card, _ = cards.GetByID(ctx, card.ID) // refresh version

	body, _ := json.Marshal(moveCardReq{ColumnID: "done", Position: 0, Version: card.Version})
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/cards/{id}/move", noopAuth(http.HandlerFunc(h.Move)))

	req := httptest.NewRequest("POST", "/api/v1/cards/"+card.ID+"/move", bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "transition_blocked" {
		t.Errorf("error: %v, want transition_blocked", resp["error"])
	}
}

func TestTransitionRequireCommentDone(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)

	settings := `{"transition_rules":{"require_comment_done":true}}`
	_, _ = boards.Update(ctx, board.ID, repo.BoardUpdate{Settings: &settings})

	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "todo", Title: "No Comments"})

	// Should fail — no comments
	body, _ := json.Marshal(moveCardReq{ColumnID: "done", Position: 0, Version: card.Version})
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/cards/{id}/move", noopAuth(http.HandlerFunc(h.Move)))

	req := httptest.NewRequest("POST", "/api/v1/cards/"+card.ID+"/move", bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 without comment, got %d", w.Code)
	}

	// Add a comment and retry
	_, _ = comments.Create(ctx, card.ID, user.ID, "Done reason")
	card, _ = cards.GetByID(ctx, card.ID)

	body2, _ := json.Marshal(moveCardReq{ColumnID: "done", Position: 0, Version: card.Version})
	req2 := httptest.NewRequest("POST", "/api/v1/cards/"+card.ID+"/move", bytes.NewReader(body2))
	req2 = withUser(req2, user)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 with comment, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestTransitionRequireAssigneeInProgress(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)

	settings := `{"transition_rules":{"require_assignee_prog":true}}`
	_, _ = boards.Update(ctx, board.ID, repo.BoardUpdate{Settings: &settings})

	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "todo", Title: "Unassigned"})

	// Should fail — no assignee
	body, _ := json.Marshal(moveCardReq{ColumnID: "in-progress", Position: 0, Version: card.Version})
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/cards/{id}/move", noopAuth(http.HandlerFunc(h.Move)))

	req := httptest.NewRequest("POST", "/api/v1/cards/"+card.ID+"/move", bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 without assignee, got %d", w.Code)
	}

	// Assign and retry
	assignee := user.ID
	assigneePtr := &assignee
	_, _ = cards.Update(ctx, card.ID, card.Version, repo.CardUpdate{AssigneeID: &assigneePtr})
	card, _ = cards.GetByID(ctx, card.ID)

	body2, _ := json.Marshal(moveCardReq{ColumnID: "in-progress", Position: 0, Version: card.Version})
	req2 := httptest.NewRequest("POST", "/api/v1/cards/"+card.ID+"/move", bytes.NewReader(body2))
	req2 = withUser(req2, user)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 with assignee, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestTransitionAllowedWithoutRules(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(cards, boards, comments, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)
	// No transition rules set — default empty settings

	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "todo", Title: "Free Move"})

	body, _ := json.Marshal(moveCardReq{ColumnID: "done", Position: 0, Version: card.Version})
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/cards/{id}/move", noopAuth(http.HandlerFunc(h.Move)))

	req := httptest.NewRequest("POST", "/api/v1/cards/"+card.ID+"/move", bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with no rules, got %d: %s", w.Code, w.Body.String())
	}
}

// ── Comment Update Test ──

func TestCommentUpdate(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)
	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "todo", Title: "Card"})
	cmt, _ := comments.Create(ctx, card.ID, user.ID, "Original text")

	// Update the comment
	updated, err := comments.Update(ctx, cmt.ID, "Edited text")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Body != "Edited text" {
		t.Errorf("body: %s, want 'Edited text'", updated.Body)
	}
	if !updated.UpdatedAt.After(updated.CreatedAt) {
		t.Errorf("updated_at should be after created_at")
	}
}
