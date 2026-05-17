package sse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/oceanplexian/lwts/server/internal/auth"
)

// wsServer mounts the websocket handler on an httptest server.
func wsServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/boards/{id}/ws", handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func wsURL(httpURL, boardID, query string) string {
	u := strings.Replace(httpURL, "http://", "ws://", 1)
	url := u + "/api/v1/boards/" + boardID + "/ws"
	if query != "" {
		url += "?" + query
	}
	return url
}

func TestWebSocket_Unauthorized(t *testing.T) {
	ds := setupTestDB(t)
	hub := startHub(t)
	users := &stubUserStore{ds: ds}
	handler := WebSocketHandler(hub, NewDBEventStore(ds), "secret", ds, users)
	srv := wsServer(t, handler)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, wsURL(srv.URL, "any", ""), nil)
	if err == nil {
		t.Fatal("expected dial error for unauthorized request")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", resp)
	}
}

func TestWebSocket_LiveAndReplay(t *testing.T) {
	ds := setupTestDB(t)
	boardID := seedBoard(t, ds)
	userID, email := seedUser(t, ds)
	store := NewDBEventStore(ds)

	// Pre-existing event we'll replay.
	pre, err := store.Persist(context.Background(), boardID, "card_created", []byte(`{"id":"a"}`), "")
	if err != nil {
		t.Fatalf("persist pre: %v", err)
	}

	hub := NewHub()
	hub.SetStore(store)
	go hub.Run()
	t.Cleanup(hub.Stop)

	users := &stubUserStore{ds: ds}
	pair, _, _ := auth.IssueTokens("secret", userID, email, "member")
	handler := WebSocketHandler(hub, store, "secret", ds, users)
	srv := wsServer(t, handler)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// last_event_id=0 means "replay everything we have."
	conn, _, err := websocket.Dial(ctx,
		wsURL(srv.URL, boardID, "token="+pair.AccessToken+"&last_event_id=0"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	read := func() map[string]any {
		var m map[string]any
		if err := wsjson.Read(ctx, conn, &m); err != nil {
			t.Fatalf("read: %v", err)
		}
		return m
	}

	// 1) connected
	if got := read(); got["type"] != "connected" {
		t.Fatalf("expected connected, got %v", got)
	}
	// 2) replayed card_created
	got := read()
	if got["type"] != "card_created" {
		t.Fatalf("expected replayed card_created, got %v", got)
	}
	if int64(got["id"].(float64)) != pre.ID {
		t.Fatalf("expected replayed id %d, got %v", pre.ID, got["id"])
	}

	// Push a live event and confirm it arrives with a fresh id.
	hub.Broadcast <- &BoardEvent{
		BoardID:   boardID,
		EventType: "card_moved",
		Data:      []byte(`{"id":"a","from_column_id":"backlog","to_column_id":"todo"}`),
	}

	live := read()
	if live["type"] != "card_moved" {
		t.Fatalf("expected live card_moved, got %v", live)
	}
	if int64(live["id"].(float64)) <= pre.ID {
		t.Fatalf("expected live id > replay id, got %v", live["id"])
	}
	payload := live["payload"].(map[string]any)
	if payload["from_column_id"] != "backlog" || payload["to_column_id"] != "todo" {
		t.Fatalf("missing from/to column on card_moved: %v", payload)
	}
}

func TestWebSocket_ResumeSkipsAlreadySeen(t *testing.T) {
	ds := setupTestDB(t)
	boardID := seedBoard(t, ds)
	userID, email := seedUser(t, ds)
	store := NewDBEventStore(ds)

	first, _ := store.Persist(context.Background(), boardID, "card_created", []byte(`{}`), "")
	second, _ := store.Persist(context.Background(), boardID, "card_updated", []byte(`{}`), "")

	hub := NewHub()
	hub.SetStore(store)
	go hub.Run()
	t.Cleanup(hub.Stop)

	users := &stubUserStore{ds: ds}
	pair, _, _ := auth.IssueTokens("secret", userID, email, "member")
	handler := WebSocketHandler(hub, store, "secret", ds, users)
	srv := wsServer(t, handler)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx,
		wsURL(srv.URL, boardID, "token="+pair.AccessToken+"&last_event_id="+strconv.FormatInt(first.ID, 10)), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	var connected, replay map[string]any
	if err := wsjson.Read(ctx, conn, &connected); err != nil {
		t.Fatalf("read connected: %v", err)
	}
	if connected["type"] != "connected" {
		t.Fatalf("expected connected, got %v", connected)
	}
	if err := wsjson.Read(ctx, conn, &replay); err != nil {
		t.Fatalf("read replay: %v", err)
	}
	if int64(replay["id"].(float64)) != second.ID {
		t.Fatalf("expected only %d to replay, got %v", second.ID, replay["id"])
	}

	// Nothing else should arrive in the short window — confirm via short timeout.
	shortCtx, shortCancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer shortCancel()
	var extra json.RawMessage
	if err := wsjson.Read(shortCtx, conn, &extra); err == nil {
		t.Fatalf("expected no further messages, got: %s", extra)
	}
}
