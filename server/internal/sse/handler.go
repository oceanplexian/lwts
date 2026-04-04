package sse

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/oceanplexian/lwts/server/internal/auth"
)

// StreamHandler returns an HTTP handler for SSE streaming on a board.
// Route: GET /api/v1/boards/{id}/stream
func StreamHandler(hub *Hub, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		boardID := r.PathValue("id")
		if boardID == "" {
			http.Error(w, `{"error":"missing board id"}`, http.StatusBadRequest)
			return
		}

		// Auth: extract JWT from Authorization header or query param
		claims, err := extractClaims(r, jwtSecret)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		client := &Client{
			BoardID:  boardID,
			UserID:   claims.Subject,
			Username: claims.Email,
			Send:     make(chan []byte, 64),
		}

		hub.Register(client)
		defer hub.Unregister(client)

		// Send initial connected event
		connData, _ := json.Marshal(map[string]string{"status": "connected", "board_id": boardID})
		_, _ = w.Write(formatSSE("connected", connData))
		flusher.Flush()

		ctx := r.Context()
		for {
			select {
			case msg, ok := <-client.Send:
				if !ok {
					// Channel closed (slow client disconnect)
					return
				}
				_, _ = w.Write(msg)
				flusher.Flush()
			case <-ctx.Done():
				return
			}
		}
	}
}

// PresenceHandler returns the list of connected users for a board.
// Route: GET /api/v1/boards/{id}/presence
func PresenceHandler(hub *Hub, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		boardID := r.PathValue("id")
		if boardID == "" {
			http.Error(w, `{"error":"missing board id"}`, http.StatusBadRequest)
			return
		}

		_, err := extractClaims(r, jwtSecret)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		users := hub.BoardPresence(boardID)
		if users == nil {
			users = []map[string]string{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	}
}

func extractClaims(r *http.Request, secret string) (*auth.AccessClaims, error) {
	// Try Authorization header first
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		return auth.ParseAccessToken(secret, token)
	}

	// Fall back to query param (for EventSource which can't set headers)
	token := r.URL.Query().Get("token")
	if token != "" {
		return auth.ParseAccessToken(secret, token)
	}

	return nil, http.ErrNoCookie
}
