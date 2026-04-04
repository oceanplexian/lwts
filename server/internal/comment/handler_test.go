package comment

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

func TestCommentCreate(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(comments, cards, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "B", user.ID)
	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "todo", Title: "Card"})

	body, _ := json.Marshal(createCommentReq{Body: "Hello world"})

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/cards/{cardId}/comments", noopAuth(http.HandlerFunc(h.Create)))

	req := httptest.NewRequest("POST", "/api/v1/cards/"+card.ID+"/comments", bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}

	var cmt repo.Comment
	json.Unmarshal(w.Body.Bytes(), &cmt)
	if cmt.Body != "Hello world" {
		t.Errorf("body: %s", cmt.Body)
	}
	if cmt.AuthorID != user.ID {
		t.Errorf("author: %s", cmt.AuthorID)
	}
}

func TestCommentList(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(comments, cards, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "B", user.ID)
	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "todo", Title: "Card"})

	comments.Create(ctx, card.ID, user.ID, "First")
	comments.Create(ctx, card.ID, user.ID, "Second")
	comments.Create(ctx, card.ID, user.ID, "Third")

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/cards/{cardId}/comments", noopAuth(http.HandlerFunc(h.ListByCard)))

	req := httptest.NewRequest("GET", "/api/v1/cards/"+card.ID+"/comments", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	var list []repo.Comment
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 3 {
		t.Fatalf("count: %d", len(list))
	}
}

func TestCommentDeleteOwn(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(comments, cards, nil)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "B", user.ID)
	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "todo", Title: "Card"})
	cmt, _ := comments.Create(ctx, card.ID, user.ID, "To delete")

	mux := http.NewServeMux()
	mux.Handle("DELETE /api/v1/comments/{id}", noopAuth(http.HandlerFunc(h.Delete)))

	req := httptest.NewRequest("DELETE", "/api/v1/comments/"+cmt.ID, nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCommentDeleteForbidden(t *testing.T) {
	users, boards, cards, comments := setupTest(t)
	h := NewHandler(comments, cards, nil)
	ctx := context.Background()

	author, _ := users.Create(ctx, "Author", "author@t.com", "h")
	other, _ := users.Create(ctx, "Other", "other@t.com", "h")
	board, _ := boards.Create(ctx, "B", "B", author.ID)
	card, _ := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "todo", Title: "Card"})
	cmt, _ := comments.Create(ctx, card.ID, author.ID, "Not yours")

	mux := http.NewServeMux()
	mux.Handle("DELETE /api/v1/comments/{id}", noopAuth(http.HandlerFunc(h.Delete)))

	req := httptest.NewRequest("DELETE", "/api/v1/comments/"+cmt.ID, nil)
	req = withUser(req, other)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}
