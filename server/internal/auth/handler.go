package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/oceanplexian/lwts/server/internal/repo"
)

// RegistrationChecker checks whether user registration is allowed.
type RegistrationChecker interface {
	IsRegistrationAllowed(r *http.Request) bool
}

// SeedFunc is called after the first user registers to populate demo data.
type SeedFunc func(ctx context.Context, ownerID string) error

// Handler holds auth endpoint dependencies.
type Handler struct {
	users     UserStore
	tokens    TokenStore
	jwtSecret string
	logger    *slog.Logger
	regCheck  RegistrationChecker
	seedFunc  SeedFunc
	ds        db.Datasource
}

func NewHandler(users UserStore, tokens TokenStore, jwtSecret string, logger *slog.Logger) *Handler {
	return &Handler{users: users, tokens: tokens, jwtSecret: jwtSecret, logger: logger}
}

func (h *Handler) SetDatasource(ds db.Datasource) {
	h.ds = ds
}

// SetRegistrationChecker sets the checker used to gate registration.
func (h *Handler) SetRegistrationChecker(rc RegistrationChecker) {
	h.regCheck = rc
}

// SetSeedFunc sets the function called after the first user registers.
func (h *Handler) SetSeedFunc(fn SeedFunc) {
	h.seedFunc = fn
}

// RegisterRoutes mounts auth routes onto the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/register", h.Register)
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.HandleFunc("POST /api/auth/refresh", h.Refresh)
	mux.HandleFunc("POST /api/auth/logout", h.Logout)
	mux.HandleFunc("GET /api/auth/me", h.Me)
	mux.HandleFunc("POST /api/auth/welcomed", h.MarkWelcomed)
	mux.HandleFunc("POST /api/auth/forgot-password", h.ForgotPassword)
	mux.HandleFunc("POST /api/auth/reset-password", h.ResetPassword)
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type authResponse struct {
	User         *repo.User `json:"user"`
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	// Check if this is first-run (0 users). If so, registration is always allowed.
	// Use CountUsers; treat errors as first-run to match RegistrationStatus behavior.
	firstRun := true
	if count, err := h.users.CountUsers(r.Context()); err == nil {
		firstRun = count == 0
	}

	if !firstRun {
		if h.regCheck != nil && !h.regCheck.IsRegistrationAllowed(r) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "user registration is disabled"})
			return
		}
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	email, err := SanitizeEmail(req.Email)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	name, err := SanitizeName(req.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := ValidatePassword(req.Password); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Check if email already exists
	existing, _ := h.users.GetUserByEmail(r.Context(), email)
	if existing != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "email already registered"})
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		h.logger.Error("hash password", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	initials := buildInitials(name)
	avatarColor := "#82B1FF"

	// First user becomes owner
	role := "member"
	if firstRun {
		role = "owner"
	}

	user, err := h.users.CreateUser(r.Context(), email, name, hash, avatarColor, initials, role)
	if err != nil {
		h.logger.Error("create user", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Set owner role explicitly (CreateUser may default to member)
	if firstRun {
		if err := h.users.UpdateUserRole(r.Context(), user.ID, "owner"); err != nil {
			h.logger.Error("set owner role", "error", err)
		} else {
			user.Role = "owner"
		}
	}

	// Seed demo data for first user
	if firstRun && h.seedFunc != nil {
		if err := h.seedFunc(r.Context(), user.ID); err != nil {
			h.logger.Error("seed demo data", "error", err)
		}
	}

	tokens, jti, err := IssueTokens(h.jwtSecret, user.ID, user.Email, user.Role)
	if err != nil {
		h.logger.Error("issue tokens", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if err := h.tokens.SaveRefreshToken(r.Context(), user.ID, jti, time.Now().Add(RefreshTokenTTL)); err != nil {
		h.logger.Error("save refresh token", "error", err)
	}

	writeJSON(w, http.StatusCreated, authResponse{User: user, AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	email, err := SanitizeEmail(req.Email)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
		return
	}

	user, err := h.users.GetUserByEmail(r.Context(), email)
	if err != nil || user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
		return
	}

	if !CheckPassword(user.PasswordHash, req.Password) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
		return
	}

	tokens, jti, err := IssueTokens(h.jwtSecret, user.ID, user.Email, user.Role)
	if err != nil {
		h.logger.Error("issue tokens", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if err := h.tokens.SaveRefreshToken(r.Context(), user.ID, jti, time.Now().Add(RefreshTokenTTL)); err != nil {
		h.logger.Error("save refresh token", "error", err)
	}

	writeJSON(w, http.StatusOK, authResponse{User: user, AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	claims, err := ParseRefreshToken(h.jwtSecret, req.RefreshToken)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired refresh token"})
		return
	}

	// Check that token exists in DB (not revoked)
	record, err := h.tokens.GetRefreshToken(r.Context(), claims.ID)
	if err != nil || record == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "refresh token revoked"})
		return
	}

	// Revoke old token
	if err := h.tokens.RevokeRefreshToken(r.Context(), claims.ID); err != nil {
		h.logger.Error("revoke refresh token", "error", err)
	}

	user, err := h.users.GetUserByID(r.Context(), claims.Subject)
	if err != nil || user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "user not found"})
		return
	}

	tokens, jti, err := IssueTokens(h.jwtSecret, user.ID, user.Email, user.Role)
	if err != nil {
		h.logger.Error("issue tokens", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if err := h.tokens.SaveRefreshToken(r.Context(), user.ID, jti, time.Now().Add(RefreshTokenTTL)); err != nil {
		h.logger.Error("save refresh token", "error", err)
	}

	writeJSON(w, http.StatusOK, authResponse{User: user, AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	if req.RefreshToken != "" {
		claims, err := ParseRefreshToken(h.jwtSecret, req.RefreshToken)
		if err == nil {
			h.tokens.RevokeRefreshToken(r.Context(), claims.ID)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing or invalid authorization header"})
		return
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

	var user *repo.User
	var err error

	if strings.HasPrefix(tokenStr, "lwts_sk_") && h.ds != nil {
		user, err = authenticateAPIKey(r.Context(), h.ds, h.users, tokenStr)
	} else {
		var claims *AccessClaims
		claims, err = ParseAccessToken(h.jwtSecret, tokenStr)
		if err == nil {
			user, err = h.users.GetUserByID(r.Context(), claims.Subject)
		}
	}

	if err != nil || user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) MarkWelcomed(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing or invalid authorization header"})
		return
	}
	claims, err := ParseAccessToken(h.jwtSecret, strings.TrimPrefix(authHeader, "Bearer "))
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}
	welcomed := true
	if _, err := h.users.UpdateUser(r.Context(), claims.Subject, repo.UserUpdate{Welcomed: &welcomed}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	// Always 200 — log reset token for dev
	if email, err := SanitizeEmail(body.Email); err == nil {
		h.logger.Info("password reset requested", "email", email, "token", "mock-reset-token-12345")
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.Token == "" || body.NewPassword == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token and new_password are required"})
		return
	}
	if err := ValidatePassword(body.NewPassword); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	// Mock implementation — just accept it
	h.logger.Info("password reset completed", "token", body.Token)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func buildInitials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "?"
	}
	if len(parts) == 1 {
		r := []rune(parts[0])
		if len(r) >= 2 {
			return strings.ToUpper(string(r[:2]))
		}
		return strings.ToUpper(string(r))
	}
	return strings.ToUpper(string([]rune(parts[0])[:1]) + string([]rune(parts[len(parts)-1])[:1]))
}
