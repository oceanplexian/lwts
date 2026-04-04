package card

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oceanplexian/lwts/server/internal/auth"
	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/oceanplexian/lwts/server/internal/repo"
	"github.com/oceanplexian/lwts/server/migrations"
)

func setupTest(t *testing.T) (*repo.UserRepository, *repo.BoardRepository, *repo.CardRepository, *repo.CommentRepository) {
	t.Helper()
	ds, err := db.NewSQLiteDatasource("sqlite://:memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ds.Close() })

	if err := db.Migrate(context.Background(), ds, migrations.FS); err != nil {
		t.Fatal(err)
	}

	return repo.NewUserRepository(ds),
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
	json.Unmarshal(w.Body.Bytes(), &card)
	if card.Key != "LWTS-1" {
		t.Errorf("key: %s", card.Key)
	}
	if card.Title != "Fix bug" {
		t.Errorf("title: %s", card.Title)
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
	json.Unmarshal(w.Body.Bytes(), &updated)
	if updated.Title != "Updated" {
		t.Errorf("title: %s", updated.Title)
	}
	if updated.Version != 2 {
		t.Errorf("version: %d", updated.Version)
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
	json.Unmarshal(w.Body.Bytes(), &moved)
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
	comments.Create(ctx, card.ID, user.ID, "Comment 1")
	comments.Create(ctx, card.ID, user.ID, "Comment 2")

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
	json.Unmarshal(w.Body.Bytes(), &resp)

	var cmts []repo.Comment
	json.Unmarshal(resp["comments"], &cmts)
	if len(cmts) != 2 {
		t.Errorf("comments: %d", len(cmts))
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
	json.Unmarshal(w.Body.Bytes(), &epic)
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
	json.Unmarshal(w.Body.Bytes(), &child)
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
	json.Unmarshal(w.Body.Bytes(), &moved)
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
	json.Unmarshal(w.Body.Bytes(), &moved)
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
	json.Unmarshal(w.Body.Bytes(), &updated)
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
	json.Unmarshal(w2.Body.Bytes(), &cleared)
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
	boards.Update(ctx, board.ID, repo.BoardUpdate{Settings: &settings})

	// Create a card with blocked dependencies
	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "todo", Title: "Blocked Card"})
	blockedIDs := `["some-blocking-card-id"]`
	cards.Update(ctx, card.ID, card.Version, repo.CardUpdate{BlockedCardIDs: &blockedIDs})
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
	json.Unmarshal(w.Body.Bytes(), &resp)
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
	boards.Update(ctx, board.ID, repo.BoardUpdate{Settings: &settings})

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
	comments.Create(ctx, card.ID, user.ID, "Done reason")
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
	boards.Update(ctx, board.ID, repo.BoardUpdate{Settings: &settings})

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
	cards.Update(ctx, card.ID, card.Version, repo.CardUpdate{AssigneeID: &assigneePtr})
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
