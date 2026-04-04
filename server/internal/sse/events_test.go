package sse

import (
	"encoding/json"
	"testing"
	"time"
)

type testCard struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Version int    `json:"version"`
}

func TestEmitCardEvent_IncludesVersion(t *testing.T) {
	h := startHub(t)

	c := newClient("board-1", "user-1", "Alice")
	h.Register(c)
	time.Sleep(50 * time.Millisecond)

	card := testCard{ID: "card-1", Title: "Test", Version: 3}
	EmitCardEvent(h, "board-1", "card_updated", card, "")

	msg, ok := drainOne(c, time.Second)
	if !ok {
		t.Fatal("did not receive event")
	}

	// Parse out the data line
	s := string(msg)
	var dataLine string
	for _, line := range splitLines(s) {
		if len(line) > 6 && line[:6] == "data: " {
			dataLine = line[6:]
		}
	}
	if dataLine == "" {
		t.Fatalf("no data line found in SSE message: %s", s)
	}

	var received testCard
	if err := json.Unmarshal([]byte(dataLine), &received); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if received.Version != 3 {
		t.Fatalf("expected version 3, got %d", received.Version)
	}
	if received.ID != "card-1" {
		t.Fatalf("expected card-1, got %s", received.ID)
	}
}

func TestEmitCardEvent_SkipsSender(t *testing.T) {
	h := startHub(t)

	sender := newClient("board-1", "user-1", "Alice")
	other := newClient("board-1", "user-2", "Bob")
	h.Register(sender)
	h.Register(other)
	time.Sleep(50 * time.Millisecond)

	// Drain user_joined events
	for i := 0; i < 4; i++ {
		select {
		case <-sender.Send:
		default:
		}
		select {
		case <-other.Send:
		default:
		}
	}
	time.Sleep(20 * time.Millisecond)

	card := testCard{ID: "card-1", Title: "Test", Version: 1}
	EmitCardEvent(h, "board-1", "card_created", card, "user-1")

	// Other should receive it
	_, ok := drainOne(other, time.Second)
	if !ok {
		t.Fatal("other did not receive event")
	}

	// Sender should NOT receive it
	select {
	case <-sender.Send:
		t.Fatal("sender received their own event")
	case <-time.After(100 * time.Millisecond):
		// good
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
