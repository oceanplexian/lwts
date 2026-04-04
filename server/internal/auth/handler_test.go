package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oceanplexian/lwts/server/internal/repo"
)

// mockTokenStore implements TokenStore for tests.
type mockTokenStore struct {
	tokens map[string]*RefreshTokenRecord
}

func newMockTokenStore() *mockTokenStore {
	return &mockTokenStore{tokens: make(map[string]*RefreshTokenRecord)}
}

func (m *mockTokenStore) SaveRefreshToken(_ context.Context, userID, jti string, expiresAt time.Time) error {
	m.tokens[jti] = &RefreshTokenRecord{UserID: userID, JTI: jti, ExpiresAt: expiresAt, CreatedAt: time.Now()}
	return nil
}

func (m *mockTokenStore) GetRefreshToken(_ context.Context, jti string) (*RefreshTokenRecord, error) {
	if r, ok := m.tokens[jti]; ok {
		return r, nil
	}
	return nil, repo.ErrNotFound
}

func (m *mockTokenStore) RevokeRefreshToken(_ context.Context, jti string) error {
	delete(m.tokens, jti)
	return nil
}

func (m *mockTokenStore) RevokeAllUserTokens(_ context.Context, userID string) error {
	for k, v := range m.tokens {
		if v.UserID == userID {
			delete(m.tokens, k)
		}
	}
	return nil
}

func newTestHandler() (*Handler, *mockUserStore, *mockTokenStore) {
	users := newMockUserStore()
	tokens := newMockTokenStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHandler(users, tokens, testSecret, logger)
	return h, users, tokens
}

func jsonBody(v any) *bytes.Reader {
	b, _ := json.Marshal(v)
	return bytes.NewReader(b)
}

func TestRegister_Success(t *testing.T) {
	h, _, _ := newTestHandler()
	body := jsonBody(map[string]string{"name": "Alice Smith", "email": "alice@test.com", "password": "Password1"})
	req := httptest.NewRequest("POST", "/api/auth/register", body)
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body: %s", rec.Code, rec.Body.String())
	}

	var resp authResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.User == nil {
		t.Fatal("user is nil")
	}
	if resp.User.Email != "alice@test.com" {
		t.Errorf("email = %q", resp.User.Email)
	}
	if resp.User.PasswordHash != "" {
		t.Error("password_hash leaked in response")
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Error("tokens missing")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	h, _, _ := newTestHandler()

	body := jsonBody(map[string]string{"name": "Alice Smith", "email": "alice@test.com", "password": "Password1"})
	req := httptest.NewRequest("POST", "/api/auth/register", body)
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first register failed: %d", rec.Code)
	}

	body = jsonBody(map[string]string{"name": "Alice Again", "email": "alice@test.com", "password": "Password2"})
	req = httptest.NewRequest("POST", "/api/auth/register", body)
	rec = httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestRegister_InvalidEmail(t *testing.T) {
	h, _, _ := newTestHandler()
	body := jsonBody(map[string]string{"name": "Alice", "email": "notanemail", "password": "Password1"})
	req := httptest.NewRequest("POST", "/api/auth/register", body)
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestRegister_WeakPassword(t *testing.T) {
	h, _, _ := newTestHandler()
	// ValidatePassword only rejects empty passwords now, so "short" is accepted
	body := jsonBody(map[string]string{"name": "Alice Smith", "email": "alice@test.com", "password": "short"})
	req := httptest.NewRequest("POST", "/api/auth/register", body)
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
}

func TestRegister_ShortName(t *testing.T) {
	h, _, _ := newTestHandler()
	body := jsonBody(map[string]string{"name": "A", "email": "alice@test.com", "password": "Password1"})
	req := httptest.NewRequest("POST", "/api/auth/register", body)
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestLogin_Success(t *testing.T) {
	h, users, _ := newTestHandler()

	// Create user first
	hash, _ := HashPassword("Password1")
	users.users["email:alice@test.com"] = &repo.User{
		ID: "user-123", Email: "alice@test.com", Name: "Alice",
		PasswordHash: hash, Role: "member",
	}
	users.users["user-123"] = users.users["email:alice@test.com"]

	body := jsonBody(map[string]string{"email": "alice@test.com", "password": "Password1"})
	req := httptest.NewRequest("POST", "/api/auth/login", body)
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}
	var resp authResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.AccessToken == "" {
		t.Error("access token missing")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	h, users, _ := newTestHandler()
	hash, _ := HashPassword("Password1")
	users.users["email:alice@test.com"] = &repo.User{
		ID: "user-123", Email: "alice@test.com", PasswordHash: hash, Role: "member",
	}

	body := jsonBody(map[string]string{"email": "alice@test.com", "password": "WrongPass1"})
	req := httptest.NewRequest("POST", "/api/auth/login", body)
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogin_NonexistentUser(t *testing.T) {
	h, _, _ := newTestHandler()
	body := jsonBody(map[string]string{"email": "nobody@test.com", "password": "Password1"})
	req := httptest.NewRequest("POST", "/api/auth/login", body)
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestRefresh_Success(t *testing.T) {
	h, users, tokens := newTestHandler()
	users.users["user-123"] = &repo.User{ID: "user-123", Email: "alice@test.com", Role: "member"}

	pair, jti, _ := IssueTokens(testSecret, "user-123", "alice@test.com", "member")
	_ = tokens.SaveRefreshToken(context.Background(), "user-123", jti, time.Now().Add(RefreshTokenTTL))

	body := jsonBody(map[string]string{"refresh_token": pair.RefreshToken})
	req := httptest.NewRequest("POST", "/api/auth/refresh", body)
	rec := httptest.NewRecorder()
	h.Refresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}

	// Old token should be revoked
	if _, err := tokens.GetRefreshToken(context.Background(), jti); err == nil {
		t.Error("old refresh token should be revoked")
	}
}

func TestRefresh_RevokedToken(t *testing.T) {
	h, _, _ := newTestHandler()
	pair, _, _ := IssueTokens(testSecret, "user-123", "alice@test.com", "member")
	// Don't save the token — it won't be found in DB

	body := jsonBody(map[string]string{"refresh_token": pair.RefreshToken})
	req := httptest.NewRequest("POST", "/api/auth/refresh", body)
	rec := httptest.NewRecorder()
	h.Refresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogout_Idempotent(t *testing.T) {
	h, _, _ := newTestHandler()

	// Logout without body
	req := httptest.NewRequest("POST", "/api/auth/logout", bytes.NewReader([]byte("{}")))
	rec := httptest.NewRecorder()
	h.Logout(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	// Logout again
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/auth/logout", bytes.NewReader([]byte("{}")))
	h.Logout(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("second logout status = %d, want 200", rec.Code)
	}
}

func TestMe_Authenticated(t *testing.T) {
	h, users, _ := newTestHandler()
	// Me() now reads the Authorization header and looks up the user via JWT,
	// not from context. Create user in mock store and issue a real token.
	users.users["user-alice@test.com"] = &repo.User{ID: "user-alice@test.com", Email: "alice@test.com", Name: "Alice", Role: "member"}

	pair, _, _ := IssueTokens(testSecret, "user-alice@test.com", "alice@test.com", "member")

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rec := httptest.NewRecorder()
	h.Me(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}
	var resp repo.User
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Email != "alice@test.com" {
		t.Errorf("email = %q", resp.Email)
	}
}

func TestMe_Unauthenticated(t *testing.T) {
	h, _, _ := newTestHandler()
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	rec := httptest.NewRecorder()
	h.Me(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestForgotPassword_Always200(t *testing.T) {
	h, _, _ := newTestHandler()
	body := jsonBody(map[string]string{"email": "anyone@test.com"})
	req := httptest.NewRequest("POST", "/api/auth/forgot-password", body)
	rec := httptest.NewRecorder()
	h.ForgotPassword(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestResetPassword_Success(t *testing.T) {
	h, _, _ := newTestHandler()
	body := jsonBody(map[string]string{"token": "mock-token", "new_password": "NewPassword1"})
	req := httptest.NewRequest("POST", "/api/auth/reset-password", body)
	rec := httptest.NewRecorder()
	h.ResetPassword(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRegister_HTMLInName(t *testing.T) {
	h, _, _ := newTestHandler()
	body := jsonBody(map[string]string{
		"name":     "<script>alert(1)</script>Alice Smith",
		"email":    "alice@test.com",
		"password": "Password1",
	})
	req := httptest.NewRequest("POST", "/api/auth/register", body)
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var resp authResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.User.Name != "alert(1)Alice Smith" {
		t.Errorf("name = %q, want 'alert(1)Alice Smith' (HTML tags stripped)", resp.User.Name)
	}
}

func TestRegister_EmailCaseNormalization(t *testing.T) {
	h, _, _ := newTestHandler()
	body := jsonBody(map[string]string{
		"name":     "Alice Smith",
		"email":    "  ALICE@TEST.COM  ",
		"password": "Password1",
	})
	req := httptest.NewRequest("POST", "/api/auth/register", body)
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var resp authResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.User.Email != "alice@test.com" {
		t.Errorf("email = %q, want 'alice@test.com'", resp.User.Email)
	}
}
