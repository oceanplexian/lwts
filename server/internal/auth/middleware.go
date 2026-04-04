package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/oceanplexian/lwts/server/internal/repo"
)

type contextKey string

const UserContextKey contextKey = "auth_user"

// RequireAuth returns middleware that validates JWT Bearer tokens or lwts_sk_ API keys.
func RequireAuth(secret string, users UserStore, ds db.Datasource) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractBearer(r)
			if tokenStr == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing or invalid authorization header"})
				return
			}

			var user *repo.User
			var err error

			if strings.HasPrefix(tokenStr, "lwts_sk_") {
				// API key authentication
				user, err = authenticateAPIKey(r.Context(), ds, users, tokenStr)
			} else {
				// JWT authentication
				var claims *AccessClaims
				claims, err = ParseAccessToken(secret, tokenStr)
				if err == nil {
					user, err = users.GetUserByID(r.Context(), claims.Subject)
				}
			}

			if err != nil || user == nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// authenticateAPIKey validates an lwts_sk_ key against the api_keys table.
func authenticateAPIKey(ctx context.Context, ds db.Datasource, users UserStore, key string) (*repo.User, error) {
	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])

	var userID string
	err := ds.QueryRow(ctx, `SELECT user_id FROM api_keys WHERE key_hash = $1`, keyHash).Scan(&userID)
	if err != nil {
		return nil, err
	}
	return users.GetUserByID(ctx, userID)
}

// RequireRole returns middleware that checks if the authenticated user has at least the given role.
func RequireRole(minRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
				return
			}
			if RoleLevel(user.Role) < RoleLevel(minRole) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UserFromContext extracts the authenticated user from the request context.
func UserFromContext(ctx context.Context) *repo.User {
	u, _ := ctx.Value(UserContextKey).(*repo.User)
	return u
}

// RoleLevel returns the numeric level for a role string.
func RoleLevel(role string) int {
	switch role {
	case "viewer":
		return 0
	case "member":
		return 1
	case "admin":
		return 2
	case "owner":
		return 3
	default:
		return -1
	}
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return h[7:]
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
