package sse

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// Client represents a connected SSE client.
type Client struct {
	BoardID  string
	UserID   string
	Username string
	Send     chan []byte // buffered, capacity 64
}

// BoardEvent is a message to broadcast to all clients on a board.
type BoardEvent struct {
	BoardID   string
	EventType string // card_created, card_updated, card_moved, card_deleted, comment_added, comment_deleted, user_joined, user_left, heartbeat
	Data      []byte // JSON payload
	SenderID  string // optional: skip this user when broadcasting
}

// Hub manages SSE clients per board.
type Hub struct {
	boards     map[string]map[*Client]bool
	mu         sync.RWMutex
	register   chan *Client
	unregister chan *Client
	Broadcast  chan *BoardEvent
	quit       chan struct{}
}

// NewHub creates a new SSE hub.
func NewHub() *Hub {
	return &Hub{
		boards:     make(map[string]map[*Client]bool),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
		Broadcast:  make(chan *BoardEvent, 256),
		quit:       make(chan struct{}),
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
			h.broadcastToBoard(client.BoardID, "user_joined", data, client.UserID)

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
			h.broadcastToBoard(client.BoardID, "user_left", data, "")

		case event := <-h.Broadcast:
			h.broadcastToBoard(event.BoardID, event.EventType, event.Data, event.SenderID)

		case <-heartbeat.C:
			h.mu.RLock()
			data := []byte("{}")
			for boardID := range h.boards {
				for client := range h.boards[boardID] {
					h.trySend(client, "heartbeat", data)
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

func (h *Hub) broadcastToBoard(boardID, eventType string, data []byte, skipUserID string) {
	h.mu.RLock()
	clients := h.boards[boardID]
	// Copy client list to avoid holding lock during sends
	targets := make([]*Client, 0, len(clients))
	for c := range clients {
		if skipUserID != "" && c.UserID == skipUserID {
			continue
		}
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		h.trySend(c, eventType, data)
	}
}

func (h *Hub) trySend(client *Client, eventType string, data []byte) {
	msg := formatSSE(eventType, data)
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
