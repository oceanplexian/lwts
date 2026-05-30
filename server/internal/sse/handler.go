package sse

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/oceanplexian/lwts/server/internal/auth"
	"github.com/oceanplexian/lwts/server/internal/db"
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

		// SSE streams are long-lived. Clear the server's WriteTimeout for this
		// response, otherwise http.Server.WriteTimeout force-closes the stream
		// after 30s and every board client reconnects on a 30s cycle.
		rc := http.NewResponseController(w)
		_ = rc.SetWriteDeadline(time.Time{})

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
		if _, err := w.Write(formatSSE("connected", connData)); err != nil {
			return
		}
		flusher.Flush()

		ctx := r.Context()
		for {
			select {
			case msg, ok := <-client.Send:
				if !ok {
					// Channel closed (slow client disconnect)
					return
				}
				if _, err := w.Write(msg); err != nil {
					// Client/connection gone — stop spinning and release resources.
					return
				}
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
		_ = json.NewEncoder(w).Encode(users)
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

// extractToken returns the bearer/query token from the request, or an empty
// string if none is present.
func extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return r.URL.Query().Get("token")
}

// extractLastEventID returns the resume cursor from either the SSE-standard
// `Last-Event-ID` header or the query string fallback. Zero means "from now."
func extractLastEventID(r *http.Request) int64 {
	if v := r.Header.Get("Last-Event-ID"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	if v := r.URL.Query().Get("last_event_id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

// EventsHandler returns an SSE handler for /api/v1/boards/{id}/events.
//
// Differences from the legacy /stream handler (which is kept untouched for the
// web client):
//   - Accepts either JWT or lwts_sk_ API key (header or ?token=)
//   - Honors Last-Event-ID (header or ?last_event_id=) to replay missed events
//   - Tags each event with an id: line so EventSource sets Last-Event-ID on reconnect
//
// If store is nil, replay is disabled and the endpoint behaves like /stream
// with API-key auth added on top.
func EventsHandler(hub *Hub, store EventStore, jwtSecret string, ds db.Datasource, users auth.UserStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		boardID := r.PathValue("id")
		if boardID == "" {
			http.Error(w, `{"error":"missing board id"}`, http.StatusBadRequest)
			return
		}

		user, err := auth.ResolveToken(r.Context(), ds, users, jwtSecret, extractToken(r))
		if err != nil || user == nil {
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

		// SSE streams are long-lived. Clear the server's WriteTimeout for this
		// response, otherwise http.Server.WriteTimeout force-closes the stream
		// after 30s and every board client reconnects on a 30s cycle.
		rc := http.NewResponseController(w)
		_ = rc.SetWriteDeadline(time.Time{})

		ctx := r.Context()
		lastSeen := extractLastEventID(r)

		// Snapshot the high-water mark BEFORE registering so live events with
		// id <= replayUntil get filtered out (the replay path will cover them).
		var replayUntil int64
		if store != nil {
			if maxID, err := store.MaxID(ctx, boardID); err == nil {
				replayUntil = maxID
			}
		}

		client := &Client{
			BoardID:   boardID,
			UserID:    user.ID,
			Username:  user.Email,
			Send:      make(chan []byte, 128),
			MinLiveID: replayUntil,
		}
		hub.Register(client)
		defer hub.Unregister(client)

		// Send initial connected event (carries the resume cursor for the client)
		connData, _ := json.Marshal(map[string]any{
			"status":           "connected",
			"board_id":         boardID,
			"last_event_id":    replayUntil,
			"replay_from":      lastSeen,
			"server_event_log": store != nil,
		})
		_, _ = w.Write(formatSSE("connected", connData))
		flusher.Flush()

		// Replay missed events
		if store != nil && replayUntil > lastSeen {
			cursor := lastSeen
			for cursor < replayUntil {
				events, err := store.LoadSince(ctx, boardID, cursor, 500)
				if err != nil || len(events) == 0 {
					break
				}
				for _, ev := range events {
					if ev.ID > replayUntil {
						break
					}
					msg := formatSSEEvent(&BoardEvent{
						ID:        ev.ID,
						BoardID:   ev.BoardID,
						EventType: ev.EventType,
						Data:      ev.Payload,
						CreatedAt: ev.CreatedAt,
					})
					if _, err := w.Write(msg); err != nil {
						return
					}
					cursor = ev.ID
				}
				flusher.Flush()
			}
		}

		for {
			select {
			case msg, ok := <-client.Send:
				if !ok {
					return
				}
				if _, err := w.Write(msg); err != nil {
					return
				}
				flusher.Flush()
			case <-ctx.Done():
				return
			}
		}
	}
}
