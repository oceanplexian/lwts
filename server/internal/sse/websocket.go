package sse

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/oceanplexian/lwts/server/internal/auth"
	"github.com/oceanplexian/lwts/server/internal/db"
)

// wsEnvelope is the per-message JSON shape sent over the WebSocket.
type wsEnvelope struct {
	ID         int64           `json:"id,omitempty"`
	Type       string          `json:"type"`
	BoardID    string          `json:"board_id"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	SenderID   string          `json:"sender_id,omitempty"`
	OccurredAt time.Time       `json:"occurred_at"`
}

func envelopeJSON(event *BoardEvent) []byte {
	env := wsEnvelope{
		ID:         event.ID,
		Type:       event.EventType,
		BoardID:    event.BoardID,
		SenderID:   event.SenderID,
		OccurredAt: event.CreatedAt,
	}
	if len(event.Data) > 0 && json.Valid(event.Data) {
		env.Payload = event.Data
	}
	buf, err := json.Marshal(env)
	if err != nil {
		return nil
	}
	return buf
}

// WebSocketHandler returns a handler for GET /api/v1/boards/{id}/ws.
//
// Auth, replay, and live-stream semantics mirror EventsHandler. Differences:
//   - The transport is a WebSocket (single full-duplex connection)
//   - Each message is a JSON envelope: {id, type, board_id, payload, occurred_at}
//   - Resume cursor comes from ?last_event_id= (no header analogue on WS)
//   - Origin checking is skipped — the API is bearer-auth, not cookie-auth
func WebSocketHandler(hub *Hub, store EventStore, jwtSecret string, ds db.Datasource, users auth.UserStore) http.HandlerFunc {
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

		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true, // bearer-auth, not cookie-auth
		})
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusInternalError, "closing")

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		lastSeen := extractLastEventID(r)
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
			OnEvent:   envelopeJSON,
		}
		hub.Register(client)
		defer hub.Unregister(client)

		// Initial connected envelope so the client learns the high-water mark.
		connPayload, _ := json.Marshal(map[string]any{
			"status":           "connected",
			"board_id":         boardID,
			"last_event_id":    replayUntil,
			"replay_from":      lastSeen,
			"server_event_log": store != nil,
		})
		connected := envelopeJSON(&BoardEvent{
			BoardID:   boardID,
			EventType: "connected",
			Data:      connPayload,
			CreatedAt: time.Now().UTC(),
		})
		if err := writeWS(ctx, conn, connected); err != nil {
			return
		}

		// Replay missed events.
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
					msg := envelopeJSON(&BoardEvent{
						ID:        ev.ID,
						BoardID:   ev.BoardID,
						EventType: ev.EventType,
						Data:      ev.Payload,
						CreatedAt: ev.CreatedAt,
					})
					if err := writeWS(ctx, conn, msg); err != nil {
						return
					}
					cursor = ev.ID
				}
			}
		}

		// Reader goroutine: drains incoming frames (we don't accept commands
		// today, but we MUST read to receive close/pong) and cancels the ctx
		// on disconnect.
		go func() {
			defer cancel()
			for {
				if _, _, err := conn.Read(ctx); err != nil {
					return
				}
			}
		}()

		// Writer loop: pump live events from the hub into the socket.
		for {
			select {
			case msg, ok := <-client.Send:
				if !ok {
					conn.Close(websocket.StatusNormalClosure, "")
					return
				}
				if err := writeWS(ctx, conn, msg); err != nil {
					return
				}
			case <-ctx.Done():
				conn.Close(websocket.StatusNormalClosure, "")
				return
			}
		}
	}
}

func writeWS(ctx context.Context, conn *websocket.Conn, msg []byte) error {
	if len(msg) == 0 {
		return nil
	}
	writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return conn.Write(writeCtx, websocket.MessageText, msg)
}
