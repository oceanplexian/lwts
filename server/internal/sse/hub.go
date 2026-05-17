package sse

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"sync"
	"time"
)

// Client represents a connected SSE/WebSocket client.
//
// MinLiveID lets richer clients (the /events SSE and /ws WebSocket transports)
// suppress live events that were already delivered via the replay path. Legacy
// clients leave it at 0 and receive everything as before.
type Client struct {
	BoardID   string
	UserID    string
	Username  string
	Send      chan []byte // buffered
	MinLiveID int64       // optional: skip live events with id <= this value
	OnEvent   func(*BoardEvent) []byte // optional: format the event for this client; falls back to formatSSE
}

// BoardEvent is a message to broadcast to all clients on a board.
//
// ID is non-zero only for events that have been persisted to board_events.
// Ephemeral events (heartbeat, user_joined, user_left, connected) keep ID == 0.
type BoardEvent struct {
	ID        int64
	BoardID   string
	EventType string // card_created, card_updated, card_moved, card_deleted, comment_added, comment_deleted, user_joined, user_left, heartbeat, connected
	Data      []byte // JSON payload
	SenderID  string // optional: skip this user when broadcasting
	CreatedAt time.Time
}

// Hub manages SSE clients per board.
type Hub struct {
	boards     map[string]map[*Client]bool
	mu         sync.RWMutex
	register   chan *Client
	unregister chan *Client
	Broadcast  chan *BoardEvent
	quit       chan struct{}
	store      EventStore
	persistCtx context.Context
}

// NewHub creates a new SSE hub.
func NewHub() *Hub {
	return &Hub{
		boards:     make(map[string]map[*Client]bool),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
		Broadcast:  make(chan *BoardEvent, 256),
		quit:       make(chan struct{}),
		persistCtx: context.Background(),
	}
}

// SetStore wires an EventStore so that broadcasts of persistable events get
// recorded (and stamped with a monotonic id) before fan-out. Nil disables
// persistence and the hub reverts to in-memory behavior.
func (h *Hub) SetStore(store EventStore) {
	h.store = store
}

// isPersistable returns whether an event type should be written to the event
// log. Presence and heartbeat events are ephemeral by design.
func isPersistable(eventType string) bool {
	switch eventType {
	case "heartbeat", "user_joined", "user_left", "connected":
		return false
	default:
		return true
	}
}

// Run starts the hub's event loop. Call in a goroutine.
func (h *Hub) Run() {
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.boards[client.BoardID] == nil {
				h.boards[client.BoardID] = make(map[*Client]bool)
			}
			h.boards[client.BoardID][client] = true
			h.mu.Unlock()

			// Broadcast user_joined to others on this board
			data, _ := json.Marshal(map[string]string{
				"user_id":  client.UserID,
				"username": client.Username,
			})
			h.broadcastEvent(&BoardEvent{
				BoardID:   client.BoardID,
				EventType: "user_joined",
				Data:      data,
				SenderID:  client.UserID,
				CreatedAt: time.Now().UTC(),
			})

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.boards[client.BoardID]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					close(client.Send)
					if len(clients) == 0 {
						delete(h.boards, client.BoardID)
					}
				}
			}
			h.mu.Unlock()

			// Broadcast user_left to remaining clients
			data, _ := json.Marshal(map[string]string{
				"user_id":  client.UserID,
				"username": client.Username,
			})
			h.broadcastEvent(&BoardEvent{
				BoardID:   client.BoardID,
				EventType: "user_left",
				Data:      data,
				CreatedAt: time.Now().UTC(),
			})

		case event := <-h.Broadcast:
			if event.CreatedAt.IsZero() {
				event.CreatedAt = time.Now().UTC()
			}
			if h.store != nil && isPersistable(event.EventType) && event.ID == 0 {
				stored, err := h.store.Persist(h.persistCtx, event.BoardID, event.EventType, event.Data, event.SenderID)
				if err != nil {
					slog.Warn("persist event failed; broadcasting anyway", "board_id", event.BoardID, "type", event.EventType, "error", err)
				} else {
					event.ID = stored.ID
					event.CreatedAt = stored.CreatedAt
				}
			}
			h.broadcastEvent(event)

		case <-heartbeat.C:
			h.mu.RLock()
			data := []byte("{}")
			hb := &BoardEvent{EventType: "heartbeat", Data: data, CreatedAt: time.Now().UTC()}
			for boardID := range h.boards {
				for client := range h.boards[boardID] {
					hb.BoardID = boardID
					h.trySend(client, hb)
				}
			}
			h.mu.RUnlock()

		case <-h.quit:
			return
		}
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Stop shuts down the hub's event loop.
func (h *Hub) Stop() {
	close(h.quit)
}

// BoardPresence returns the list of connected users for a board.
func (h *Hub) BoardPresence(boardID string) []map[string]string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := h.boards[boardID]
	seen := make(map[string]bool)
	var users []map[string]string

	for client := range clients {
		if seen[client.UserID] {
			continue
		}
		seen[client.UserID] = true
		users = append(users, map[string]string{
			"id":       client.UserID,
			"username": client.Username,
		})
	}
	return users
}

// ClientCount returns the number of connected clients for a board (for testing).
func (h *Hub) ClientCount(boardID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.boards[boardID])
}

func (h *Hub) broadcastEvent(event *BoardEvent) {
	h.mu.RLock()
	clients := h.boards[event.BoardID]
	// Copy client list to avoid holding lock during sends
	targets := make([]*Client, 0, len(clients))
	for c := range clients {
		if event.SenderID != "" && c.UserID == event.SenderID {
			continue
		}
		if event.ID != 0 && c.MinLiveID != 0 && event.ID <= c.MinLiveID {
			// Replay already delivered this event to this client.
			continue
		}
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		h.trySend(c, event)
	}
}

func (h *Hub) trySend(client *Client, event *BoardEvent) {
	var msg []byte
	if client.OnEvent != nil {
		msg = client.OnEvent(event)
	} else {
		msg = formatSSEEvent(event)
	}
	if len(msg) == 0 {
		return
	}
	select {
	case client.Send <- msg:
	default:
		// Slow client — force disconnect
		slog.Warn("slow client disconnected", "board_id", client.BoardID, "user_id", client.UserID)
		h.mu.Lock()
		if clients, ok := h.boards[client.BoardID]; ok {
			if _, exists := clients[client]; exists {
				delete(clients, client)
				close(client.Send)
				if len(clients) == 0 {
					delete(h.boards, client.BoardID)
				}
			}
		}
		h.mu.Unlock()
	}
}

func formatSSE(eventType string, data []byte) []byte {
	// Format: "event: <type>\ndata: <json>\n\n"
	buf := make([]byte, 0, len(eventType)+len(data)+20)
	buf = append(buf, "event: "...)
	buf = append(buf, eventType...)
	buf = append(buf, '\n')
	buf = append(buf, "data: "...)
	buf = append(buf, data...)
	buf = append(buf, '\n', '\n')
	return buf
}

// formatSSEEvent formats a BoardEvent as an SSE message. When the event has a
// persisted ID it is emitted as an `id:` line so EventSource clients populate
// Last-Event-ID for reconnect.
func formatSSEEvent(event *BoardEvent) []byte {
	buf := make([]byte, 0, len(event.EventType)+len(event.Data)+40)
	if event.ID != 0 {
		buf = append(buf, "id: "...)
		buf = strconv.AppendInt(buf, event.ID, 10)
		buf = append(buf, '\n')
	}
	buf = append(buf, "event: "...)
	buf = append(buf, event.EventType...)
	buf = append(buf, '\n')
	buf = append(buf, "data: "...)
	buf = append(buf, event.Data...)
	buf = append(buf, '\n', '\n')
	return buf
}
