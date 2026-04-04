package board

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

func setupTest(t *testing.T) (db.Datasource, *repo.UserRepository, *repo.BoardRepository, *repo.CardRepository, *repo.CommentRepository) {
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

func createTestUser(t *testing.T, users *repo.UserRepository) repo.User {
	t.Helper()
	u, err := users.Create(context.Background(), "Test User", "test@test.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func withUser(r *http.Request, u repo.User) *http.Request {
	ctx := context.WithValue(r.Context(), auth.UserContextKey, &u)
	return r.WithContext(ctx)
}

func noopAuth(next http.Handler) http.Handler { return next }

func TestBoardCreate(t *testing.T) {
	_, users, boards, cards, comments := setupTest(t)
	h := NewHandler(boards, cards, comments, nil)
	user := createTestUser(t, users)

	body, _ := json.Marshal(createBoardReq{Name: "Sprint 1", ProjectKey: "SP"})
	req := httptest.NewRequest("POST", "/api/v1/boards", bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}

	var b repo.Board
	_ = json.Unmarshal(w.Body.Bytes(), &b)
	if b.Name != "Sprint 1" {
		t.Errorf("name: %s", b.Name)
	}
	if b.ProjectKey != "SP" {
		t.Errorf("project_key: %s", b.ProjectKey)
	}
}

func TestBoardCreateMissingName(t *testing.T) {
	_, users, boards, cards, comments := setupTest(t)
	h := NewHandler(boards, cards, comments, nil)
	user := createTestUser(t, users)

	body, _ := json.Marshal(createBoardReq{})
	req := httptest.NewRequest("POST", "/api/v1/boards", bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()

	h.Create(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestBoardList(t *testing.T) {
	_, users, boards, cards, comments := setupTest(t)
	h := NewHandler(boards, cards, comments, nil)
	user := createTestUser(t, users)
	ctx := context.Background()

	_, _ = boards.Create(ctx, "B1", "B1", user.ID)
	_, _ = boards.Create(ctx, "B2", "B2", user.ID)

	req := httptest.NewRequest("GET", "/api/v1/boards", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()

	h.List(w, req)

	var list []repo.Board
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 2 {
		t.Fatalf("count: %d", len(list))
	}
}

func TestBoardGet(t *testing.T) {
	_, users, boards, cards, comments := setupTest(t)
	h := NewHandler(boards, cards, comments, nil)
	user := createTestUser(t, users)
	ctx := context.Background()

	b, _ := boards.Create(ctx, "B", "B", user.ID)
	_, _ = cards.Create(ctx, b.ID, repo.CardCreate{ColumnID: "todo", Title: "C1"})
	_, _ = cards.Create(ctx, b.ID, repo.CardCreate{ColumnID: "todo", Title: "C2"})
	_, _ = cards.Create(ctx, b.ID, repo.CardCreate{ColumnID: "done", Title: "C3"})

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/boards/{id}", noopAuth(http.HandlerFunc(h.Get)))

	req := httptest.NewRequest("GET", "/api/v1/boards/"+b.ID, nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}

	var resp map[string]json.RawMessage
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	var counts map[string]int
	_ = json.Unmarshal(resp["card_counts"], &counts)
	if counts["todo"] != 2 {
		t.Errorf("todo count: %d", counts["todo"])
	}
	if counts["done"] != 1 {
		t.Errorf("done count: %d", counts["done"])
	}
}

func TestBoardGetNotFound(t *testing.T) {
	_, users, boards, cards, comments := setupTest(t)
	h := NewHandler(boards, cards, comments, nil)
	user := createTestUser(t, users)

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/boards/{id}", noopAuth(http.HandlerFunc(h.Get)))

	req := httptest.NewRequest("GET", "/api/v1/boards/nonexistent", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestBoardDeleteOwner(t *testing.T) {
	_, users, boards, cards, comments := setupTest(t)
	h := NewHandler(boards, cards, comments, nil)
	user := createTestUser(t, users)
	ctx := context.Background()

	b, _ := boards.Create(ctx, "B", "B", user.ID)

	mux := http.NewServeMux()
	mux.Handle("DELETE /api/v1/boards/{id}", noopAuth(http.HandlerFunc(h.Delete)))

	req := httptest.NewRequest("DELETE", "/api/v1/boards/"+b.ID, nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestBoardDeleteForbidden(t *testing.T) {
	_, users, boards, cards, comments := setupTest(t)
	h := NewHandler(boards, cards, comments, nil)
	owner := createTestUser(t, users)
	ctx := context.Background()

	other, _ := users.Create(ctx, "Other", "other@test.com", "hash")

	b, _ := boards.Create(ctx, "B", "B", owner.ID)

	mux := http.NewServeMux()
	mux.Handle("DELETE /api/v1/boards/{id}", noopAuth(http.HandlerFunc(h.Delete)))

	req := httptest.NewRequest("DELETE", "/api/v1/boards/"+b.ID, nil)
	req = withUser(req, other)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}
