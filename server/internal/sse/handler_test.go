package sse

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oceanplexian/lwts/server/internal/auth"
	"github.com/oceanplexian/lwts/server/internal/middleware"
)

// deadlineRecorder is a ResponseWriter that records SetWriteDeadline calls so a
// test can assert the SSE handler cleared the server WriteTimeout. It also
// implements http.Flusher so the handler's streaming path is exercised.
type deadlineRecorder struct {
	mu           sync.Mutex
	header       http.Header
	body         bytes.Buffer
	deadlineSet  bool
	zeroDeadline bool
}

func (d *deadlineRecorder) Header() http.Header {
	if d.header == nil {
		d.header = http.Header{}
	}
	return d.header
}

func (d *deadlineRecorder) Write(b []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.body.Write(b)
}

func (d *deadlineRecorder) WriteHeader(int) {}

func (d *deadlineRecorder) Flush() {}

func (d *deadlineRecorder) SetWriteDeadline(t time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.deadlineSet = true
	d.zeroDeadline = t.IsZero()
	return nil
}

// TestStreamHandler_ClearsWriteDeadline guards the fix for long-lived SSE
// streams being killed by http.Server.WriteTimeout. The handler must clear the
// write deadline, and it must do so through the logger middleware's wrapper
// (which requires statusRecorder.Unwrap to forward to the real connection).
func TestStreamHandler_ClearsWriteDeadline(t *testing.T) {
	hub := startHub(t)
	token := issueTestToken(t, "test-secret", "user-1", "alice@test.com")

	// Wrap with the real Logger middleware so we also verify Unwrap traversal.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := middleware.Logger(logger)(StreamHandler(hub, "test-secret"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("GET", "/api/v1/boards/board-1/stream", nil).WithContext(ctx)
	req.SetPathValue("id", "board-1")
	req.Header.Set("Authorization", "Bearer "+token)

	rec := &deadlineRecorder{}
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(rec, req)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if !rec.deadlineSet {
		t.Fatal("SSE handler did not call SetWriteDeadline (WriteTimeout would kill the stream)")
	}
	if !rec.zeroDeadline {
		t.Fatal("SSE handler set a non-zero write deadline; long-lived streams will be force-closed")
	}
}

func issueTestToken(t *testing.T, secret, userID, email string) string {
	t.Helper()
	pair, _, err := auth.IssueTokens(secret, userID, email, "member")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return pair.AccessToken
}

func TestStreamHandler_Unauthorized(t *testing.T) {
	hub := startHub(t)
	handler := StreamHandler(hub, "test-secret")

	req := httptest.NewRequest("GET", "/api/v1/boards/board-1/stream", nil)
	req.SetPathValue("id", "board-1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestStreamHandler_Headers(t *testing.T) {
	hub := startHub(t)
	handler := StreamHandler(hub, "test-secret")
	token := issueTestToken(t, "test-secret", "user-1", "alice@test.com")

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest("GET", "/api/v1/boards/board-1/stream", nil).WithContext(ctx)
	req.SetPathValue("id", "board-1")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	result := w.Result()
	if ct := result.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected Content-Type text/event-stream, got %s", ct)
	}
	if cc := result.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("expected Cache-Control no-cache, got %s", cc)
	}
	if conn := result.Header.Get("Connection"); conn != "keep-alive" {
		t.Fatalf("expected Connection keep-alive, got %s", conn)
	}
}

func TestStreamHandler_ReceivesEvents(t *testing.T) {
	hub := startHub(t)
	handler := StreamHandler(hub, "test-secret")
	token := issueTestToken(t, "test-secret", "user-1", "alice@test.com")

	// Use a pipe to simulate SSE streaming
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest("GET", "/api/v1/boards/board-1/stream", nil).WithContext(ctx)
	req.SetPathValue("id", "board-1")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(w, req)
		close(done)
	}()

	// Wait for registration
	time.Sleep(100 * time.Millisecond)

	// Broadcast an event
	hub.Broadcast <- &BoardEvent{
		BoardID:   "board-1",
		EventType: "card_created",
		Data:      []byte(`{"id":"card-1","title":"Test"}`),
	}

	time.Sleep(200 * time.Millisecond)
	cancel()
	<-done

	body := w.Body.String()
	scanner := bufio.NewScanner(strings.NewReader(body))
	foundConnected := false
	foundCardCreated := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "event: connected") {
			foundConnected = true
		}
		if strings.Contains(line, "event: card_created") {
			foundCardCreated = true
		}
	}

	if !foundConnected {
		t.Fatal("did not receive connected event")
	}
	if !foundCardCreated {
		t.Fatal("did not receive card_created event")
	}
}

func TestStreamHandler_QueryParamAuth(t *testing.T) {
	hub := startHub(t)
	handler := StreamHandler(hub, "test-secret")
	token := issueTestToken(t, "test-secret", "user-1", "alice@test.com")

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest("GET", "/api/v1/boards/board-1/stream?token="+token, nil).WithContext(ctx)
	req.SetPathValue("id", "board-1")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	result := w.Result()
	if result.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("expected SSE stream with query param auth, got status %d", w.Code)
	}
}

func TestPresenceHandler(t *testing.T) {
	hub := startHub(t)

	// Register clients directly
	c1 := newClient("board-1", "user-1", "Alice")
	c2 := newClient("board-1", "user-2", "Bob")
	hub.Register(c1)
	hub.Register(c2)
	time.Sleep(50 * time.Millisecond)

	handler := PresenceHandler(hub, "test-secret")
	token := issueTestToken(t, "test-secret", "user-1", "alice@test.com")

	req := httptest.NewRequest("GET", "/api/v1/boards/board-1/presence", nil)
	req.SetPathValue("id", "board-1")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Alice") || !strings.Contains(body, "Bob") {
		t.Fatalf("expected both users in presence response, got: %s", body)
	}
}

func TestPresenceHandler_Unauthorized(t *testing.T) {
	hub := startHub(t)
	handler := PresenceHandler(hub, "test-secret")

	req := httptest.NewRequest("GET", "/api/v1/boards/board-1/presence", nil)
	req.SetPathValue("id", "board-1")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
