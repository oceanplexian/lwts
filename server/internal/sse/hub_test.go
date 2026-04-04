package sse

import (
	"testing"
	"time"
)

func startHub(t *testing.T) *Hub {
	t.Helper()
	h := NewHub()
	go h.Run()
	t.Cleanup(func() { h.Stop() })
	return h
}

func newClient(boardID, userID, username string) *Client {
	return &Client{
		BoardID:  boardID,
		UserID:   userID,
		Username: username,
		Send:     make(chan []byte, 64),
	}
}

func drainOne(c *Client, timeout time.Duration) ([]byte, bool) {
	select {
	case msg := <-c.Send:
		return msg, true
	case <-time.After(timeout):
		return nil, false
	}
}

func TestRegisterUnregister(t *testing.T) {
	h := startHub(t)

	c1 := newClient("board-1", "user-1", "Alice")
	h.Register(c1)
	time.Sleep(50 * time.Millisecond)

	if got := h.ClientCount("board-1"); got != 1 {
		t.Fatalf("expected 1 client, got %d", got)
	}

	h.Unregister(c1)
	time.Sleep(50 * time.Millisecond)

	if got := h.ClientCount("board-1"); got != 0 {
		t.Fatalf("expected 0 clients, got %d", got)
	}
}

func TestBroadcastToBoard(t *testing.T) {
	h := startHub(t)

	c1 := newClient("board-1", "user-1", "Alice")
	c2 := newClient("board-1", "user-2", "Bob")
	c3 := newClient("board-1", "user-3", "Charlie")

	h.Register(c1)
	h.Register(c2)
	h.Register(c3)
	time.Sleep(50 * time.Millisecond)

	// Drain user_joined events
	for i := 0; i < 6; i++ { // c1 sees c2+c3 join, c2 sees c3 join, etc.
		// drain from all three, non-blocking
		select {
		case <-c1.Send:
		default:
		}
		select {
		case <-c2.Send:
		default:
		}
		select {
		case <-c3.Send:
		default:
		}
	}
	time.Sleep(20 * time.Millisecond)

	h.Broadcast <- &BoardEvent{
		BoardID:   "board-1",
		EventType: "card_created",
		Data:      []byte(`{"id":"card-1"}`),
	}

	for _, c := range []*Client{c1, c2, c3} {
		msg, ok := drainOne(c, time.Second)
		if !ok {
			t.Fatalf("client %s did not receive broadcast", c.Username)
		}
		if len(msg) == 0 {
			t.Fatalf("client %s received empty message", c.Username)
		}
	}
}

func TestCrossBoardIsolation(t *testing.T) {
	h := startHub(t)

	cA := newClient("board-A", "user-1", "Alice")
	cB := newClient("board-B", "user-2", "Bob")

	h.Register(cA)
	h.Register(cB)
	time.Sleep(50 * time.Millisecond)

	h.Broadcast <- &BoardEvent{
		BoardID:   "board-B",
		EventType: "card_deleted",
		Data:      []byte(`{"id":"x"}`),
	}
	time.Sleep(50 * time.Millisecond)

	// cA should NOT receive anything (no user_joined since different board)
	select {
	case <-cA.Send:
		t.Fatal("client on board-A received event for board-B")
	default:
		// good
	}

	// cB should receive it
	msg, ok := drainOne(cB, time.Second)
	if !ok {
		t.Fatal("client on board-B did not receive broadcast")
	}
	if len(msg) == 0 {
		t.Fatal("received empty message")
	}
}

func TestSlowClientDisconnect(t *testing.T) {
	h := startHub(t)

	// Create client with full send channel
	slow := &Client{
		BoardID:  "board-1",
		UserID:   "slow-user",
		Username: "Slowpoke",
		Send:     make(chan []byte, 2), // very small buffer
	}

	h.Register(slow)
	time.Sleep(50 * time.Millisecond)

	// Fill the buffer
	slow.Send <- []byte("a")
	slow.Send <- []byte("b")

	// Broadcast should trigger slow client disconnect
	h.Broadcast <- &BoardEvent{
		BoardID:   "board-1",
		EventType: "card_updated",
		Data:      []byte(`{"id":"card-1"}`),
	}
	time.Sleep(100 * time.Millisecond)

	if got := h.ClientCount("board-1"); got != 0 {
		t.Fatalf("expected slow client to be disconnected, got %d clients", got)
	}
}

func TestUserJoinedLeft(t *testing.T) {
	h := startHub(t)

	c1 := newClient("board-1", "user-1", "Alice")
	h.Register(c1)
	time.Sleep(50 * time.Millisecond)

	c2 := newClient("board-1", "user-2", "Bob")
	h.Register(c2)
	time.Sleep(50 * time.Millisecond)

	// c1 should receive user_joined for c2
	msg, ok := drainOne(c1, time.Second)
	if !ok {
		t.Fatal("c1 did not receive user_joined")
	}
	if !contains(msg, "user_joined") {
		t.Fatalf("expected user_joined event, got: %s", msg)
	}

	h.Unregister(c2)
	time.Sleep(50 * time.Millisecond)

	// c1 should receive user_left for c2
	msg, ok = drainOne(c1, time.Second)
	if !ok {
		t.Fatal("c1 did not receive user_left")
	}
	if !contains(msg, "user_left") {
		t.Fatalf("expected user_left event, got: %s", msg)
	}
}

func TestBoardPresence(t *testing.T) {
	h := startHub(t)

	c1 := newClient("board-1", "user-1", "Alice")
	c2 := newClient("board-1", "user-2", "Bob")
	h.Register(c1)
	h.Register(c2)
	time.Sleep(50 * time.Millisecond)

	users := h.BoardPresence("board-1")
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	// Empty board
	users = h.BoardPresence("nonexistent")
	if len(users) != 0 {
		t.Fatalf("expected 0 users for nonexistent board, got %d", len(users))
	}
}

func TestFormatSSE(t *testing.T) {
	msg := formatSSE("card_created", []byte(`{"id":"1"}`))
	expected := "event: card_created\ndata: {\"id\":\"1\"}\n\n"
	if string(msg) != expected {
		t.Fatalf("unexpected SSE format:\ngot:  %q\nwant: %q", msg, expected)
	}
}

func contains(b []byte, substr string) bool {
	return len(b) > 0 && len(substr) > 0 && stringContains(string(b), substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
