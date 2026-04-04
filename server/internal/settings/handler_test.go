package settings

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

func setupTest(t *testing.T) (*Handler, *repo.UserRepository, *repo.BoardRepository) {
	t.Helper()
	ds, err := db.NewSQLiteDatasource("sqlite://:memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ds.Close() })

	if err := db.Migrate(context.Background(), ds, migrations.FS); err != nil {
		t.Fatal(err)
	}

	users := repo.NewUserRepository(ds)
	boards := repo.NewBoardRepository(ds)
	cards := repo.NewCardRepository(ds)
	comments := repo.NewCommentRepository(ds)
	h := NewHandler(ds, users, boards, cards, comments)
	return h, users, boards
}

func withUser(r *http.Request, u repo.User) *http.Request {
	ctx := context.WithValue(r.Context(), auth.UserContextKey, &u)
	return r.WithContext(ctx)
}

func noopAuth(next http.Handler) http.Handler { return next }

func TestGetSettingsDefault(t *testing.T) {
	h, _, _ := setupTest(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, noopAuth, noopAuth)

	user := repo.User{ID: "u1", Role: "admin"}
	req := httptest.NewRequest("GET", "/api/v1/settings/general", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}
	var result map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if result["workspace_name"] != "LWTS" {
		t.Errorf("expected LWTS default, got %v", result["workspace_name"])
	}
}

func TestPutAndGetSettings(t *testing.T) {
	h, _, _ := setupTest(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, noopAuth, noopAuth)

	user := repo.User{ID: "u1", Role: "admin"}

	// PUT to update workspace name
	body, _ := json.Marshal(map[string]any{"workspace_name": "TestWorkspace"})
	req := httptest.NewRequest("PUT", "/api/v1/settings/general", bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("put status: %d, body: %s", w.Code, w.Body.String())
	}

	// GET to verify merge
	req = httptest.NewRequest("GET", "/api/v1/settings/general", nil)
	req = withUser(req, user)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["workspace_name"] != "TestWorkspace" {
		t.Errorf("workspace_name: %v", result["workspace_name"])
	}
	// Other defaults should be preserved
	if result["auto_save"] != true {
		t.Errorf("auto_save should be preserved, got %v", result["auto_save"])
	}
}

func TestSettingsUnknownCategory(t *testing.T) {
	h, _, _ := setupTest(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, noopAuth, noopAuth)

	user := repo.User{ID: "u1", Role: "admin"}
	req := httptest.NewRequest("GET", "/api/v1/settings/bogus", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAPIKeyCreateListDelete(t *testing.T) {
	h, users, _ := setupTest(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, noopAuth, noopAuth)
	ctx := context.Background()

	user, _ := users.Create(ctx, "Admin", "admin@t.com", "h")
	user.Role = "admin"

	// Create key
	body, _ := json.Marshal(map[string]string{"name": "Test Key"})
	req := httptest.NewRequest("POST", "/api/v1/keys", bytes.NewReader(body))
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create status: %d, body: %s", w.Code, w.Body.String())
	}
	var created map[string]string
	json.Unmarshal(w.Body.Bytes(), &created)
	if created["key"] == "" {
		t.Error("key should be returned on create")
	}
	if created["name"] != "Test Key" {
		t.Errorf("name: %s", created["name"])
	}
	keyID := created["id"]

	// List keys
	req = httptest.NewRequest("GET", "/api/v1/keys", nil)
	req = withUser(req, user)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var keys []map[string]any
	json.Unmarshal(w.Body.Bytes(), &keys)
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}

	// Delete key
	req = httptest.NewRequest("DELETE", "/api/v1/keys/"+keyID, nil)
	req = withUser(req, user)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status: %d", w.Code)
	}

	// Verify empty list
	req = httptest.NewRequest("GET", "/api/v1/keys", nil)
	req = withUser(req, user)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &keys)
	if len(keys) != 0 {
		t.Fatalf("expected 0 keys after delete, got %d", len(keys))
	}
}

func TestExport(t *testing.T) {
	h, users, boards := setupTest(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, noopAuth, noopAuth)
	ctx := context.Background()

	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	_, _ = boards.Create(ctx, "Board1", "B1", user.ID)

	req := httptest.NewRequest("GET", "/api/v1/export", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}

	var export map[string]any
	json.Unmarshal(w.Body.Bytes(), &export)
	if export["exported_at"] == nil {
		t.Error("missing exported_at")
	}
	boardsArr, ok := export["boards"].([]any)
	if !ok || len(boardsArr) != 1 {
		t.Errorf("expected 1 board in export, got %v", export["boards"])
	}
}

func TestResetWorkspace(t *testing.T) {
	h, users, boards := setupTest(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, noopAuth, noopAuth)
	ctx := context.Background()

	user, _ := users.Create(ctx, "Admin", "a@t.com", "h")
	_, _ = boards.Create(ctx, "Board1", "B1", user.ID)

	req := httptest.NewRequest("POST", "/api/v1/settings/reset", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}

	// Verify boards are gone
	list, _ := boards.List(ctx)
	if len(list) != 0 {
		t.Errorf("expected 0 boards after reset, got %d", len(list))
	}
}
