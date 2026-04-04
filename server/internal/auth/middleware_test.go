package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oceanplexian/lwts/server/internal/repo"
)

// mockUserStore implements UserStore for tests.
type mockUserStore struct {
	users map[string]*repo.User
}

func newMockUserStore() *mockUserStore {
	return &mockUserStore{users: make(map[string]*repo.User)}
}

func (m *mockUserStore) CreateUser(_ context.Context, email, name, passwordHash, avatarColor, initials, role string) (*repo.User, error) {
	u := &repo.User{
		ID: "user-" + email, Email: email, Name: name,
		PasswordHash: passwordHash, AvatarColor: avatarColor,
		Initials: initials, Role: role,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	m.users[u.ID] = u
	m.users["email:"+email] = u
	return u, nil
}

func (m *mockUserStore) GetUserByEmail(_ context.Context, email string) (*repo.User, error) {
	if u, ok := m.users["email:"+email]; ok {
		return u, nil
	}
	return nil, repo.ErrNotFound
}

func (m *mockUserStore) GetUserByID(_ context.Context, id string) (*repo.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, repo.ErrNotFound
}

func (m *mockUserStore) CountUsers(_ context.Context) (int, error) {
	count := 0
	for k := range m.users {
		if len(k) > 6 && k[:6] == "email:" {
			continue
		}
		count++
	}
	return count, nil
}

func (m *mockUserStore) UpdateUserRole(_ context.Context, id, role string) error {
	if u, ok := m.users[id]; ok {
		u.Role = role
		return nil
	}
	return repo.ErrNotFound
}

func (m *mockUserStore) UpdateUser(_ context.Context, id string, fields repo.UserUpdate) (*repo.User, error) {
	if u, ok := m.users[id]; ok {
		if fields.Welcomed != nil {
			u.Welcomed = *fields.Welcomed
		}
		return u, nil
	}
	return nil, repo.ErrNotFound
}

func TestRequireAuth_ValidToken(t *testing.T) {
	store := newMockUserStore()
	store.users["user-123"] = &repo.User{ID: "user-123", Email: "alice@test.com", Role: "member"}

	pair, _, _ := IssueTokens(testSecret, "user-123", "alice@test.com", "member")

	var gotUser *repo.User
	handler := RequireAuth(testSecret, store, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if gotUser == nil || gotUser.ID != "user-123" {
		t.Error("user not attached to context")
	}
}

func TestRequireAuth_MissingToken(t *testing.T) {
	store := newMockUserStore()
	handler := RequireAuth(testSecret, store, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestRequireAuth_InvalidToken(t *testing.T) {
	store := newMockUserStore()
	handler := RequireAuth(testSecret, store, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestRequireAuth_ExpiredToken(t *testing.T) {
	store := newMockUserStore()
	// Issue token with wrong secret to simulate expired
	handler := RequireAuth(testSecret, store, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	pair, _, _ := IssueTokens("different-secret", "user-123", "alice@test.com", "member")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestRequireRole_Sufficient(t *testing.T) {
	handler := RequireRole("member")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	ctx := context.WithValue(req.Context(), UserContextKey, &repo.User{ID: "1", Role: "admin"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req.WithContext(ctx))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRequireRole_Insufficient(t *testing.T) {
	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	ctx := context.WithValue(req.Context(), UserContextKey, &repo.User{ID: "1", Role: "viewer"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req.WithContext(ctx))

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestRoleLevel(t *testing.T) {
	tests := []struct {
		role string
		want int
	}{
		{"viewer", 0},
		{"member", 1},
		{"admin", 2},
		{"owner", 3},
		{"unknown", -1},
	}
	for _, tt := range tests {
		if got := RoleLevel(tt.role); got != tt.want {
			t.Errorf("RoleLevel(%q) = %d, want %d", tt.role, got, tt.want)
		}
	}
}
