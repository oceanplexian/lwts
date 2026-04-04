package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/oceanplexian/lwts/server/migrations"
)

func setupTestDB(t *testing.T) db.Datasource {
	t.Helper()
	ds, err := db.NewSQLiteDatasource("sqlite://:memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	ctx := context.Background()
	if err := db.Migrate(ctx, ds, migrations.FS); err != nil {
		ds.Close()
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { ds.Close() })
	return ds
}

// createTestBoard inserts a user + board for FK constraints.
func createTestBoard(t *testing.T, ds db.Datasource) string {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	userID := "test-user-id"
	boardID := "test-board-id"
	ds.Exec(ctx, `INSERT INTO users (id, email, name, password_hash, avatar_color, initials, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		userID, "test@test.com", "Test", "hash", "#82B1FF", "T", "owner", now, now)
	ds.Exec(ctx, `INSERT INTO boards (id, name, project_key, owner_id, columns, settings, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		boardID, "Test Board", "TST", userID, "[]", "{}", now, now)
	return boardID
}

// ── HMAC Signing ──

func TestSign(t *testing.T) {
	secret := "mysecret"
	body := []byte(`{"event":"card.created"}`)

	sig := Sign(secret, body)

	// Verify format
	if !strings.HasPrefix(sig, "sha256=") {
		t.Fatalf("signature should start with sha256=, got %q", sig)
	}

	// Verify manually
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if sig != expected {
		t.Errorf("sig = %q, want %q", sig, expected)
	}
}

func TestVerify(t *testing.T) {
	secret := "testsecret"
	body := []byte(`hello world`)
	sig := Sign(secret, body)

	if !Verify(secret, body, sig) {
		t.Error("verify should pass for correct signature")
	}
	if Verify("wrongsecret", body, sig) {
		t.Error("verify should fail for wrong secret")
	}
	if Verify(secret, []byte("tampered"), sig) {
		t.Error("verify should fail for tampered body")
	}
}

// ── Event Types ──

func TestAllEventTypesDefined(t *testing.T) {
	if len(AllEventTypes) != 6 {
		t.Errorf("expected 6 event types, got %d", len(AllEventTypes))
	}
	expected := map[string]bool{
		"card.created": true, "card.updated": true, "card.moved": true,
		"card.completed": true, "card.deleted": true, "comment.added": true,
	}
	for _, et := range AllEventTypes {
		if !expected[et] {
			t.Errorf("unexpected event type: %s", et)
		}
	}
}

// ── Store ──

func TestWebhookCRUD(t *testing.T) {
	ds := setupTestDB(t)
	boardID := createTestBoard(t, ds)
	store := NewStore(ds)
	ctx := context.Background()

	// Create
	wh, err := store.CreateWebhook(ctx, boardID, "https://example.com/hook", EventCardCreated)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(wh.Secret) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("secret len = %d, want 64", len(wh.Secret))
	}
	if !wh.Enabled {
		t.Error("should be enabled by default")
	}

	// List
	webhooks, err := store.ListWebhooks(ctx, boardID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(webhooks) != 1 {
		t.Fatalf("len = %d, want 1", len(webhooks))
	}

	// Get
	got, err := store.GetWebhook(ctx, wh.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.URL != "https://example.com/hook" {
		t.Errorf("url = %q", got.URL)
	}

	// Update: disable
	disabled := false
	updated, err := store.UpdateWebhook(ctx, wh.ID, WebhookUpdate{Enabled: &disabled})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Enabled {
		t.Error("should be disabled after update")
	}

	// Delete
	if err := store.DeleteWebhook(ctx, wh.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = store.GetWebhook(ctx, wh.ID)
	if err != db.ErrNoRows {
		t.Errorf("expected ErrNoRows after delete, got %v", err)
	}
}

func TestListEnabledForBoard(t *testing.T) {
	ds := setupTestDB(t)
	boardID := createTestBoard(t, ds)
	store := NewStore(ds)
	ctx := context.Background()

	store.CreateWebhook(ctx, boardID, "https://a.com/hook", EventCardCreated)
	store.CreateWebhook(ctx, boardID, "https://b.com/hook", EventWildcard)
	wh3, _ := store.CreateWebhook(ctx, boardID, "https://c.com/hook", EventCommentAdded)

	// Disable one
	disabled := false
	store.UpdateWebhook(ctx, wh3.ID, WebhookUpdate{Enabled: &disabled})

	// Query for card.created — should match: a (exact) + b (wildcard), not c (disabled, wrong type)
	matched, err := store.ListEnabledForBoard(ctx, boardID, EventCardCreated)
	if err != nil {
		t.Fatalf("list enabled: %v", err)
	}
	if len(matched) != 2 {
		t.Errorf("matched = %d, want 2", len(matched))
	}
}

// ── Dispatcher ──

func TestDispatcherEmitNonBlocking(t *testing.T) {
	store := NewStore(setupTestDB(t))
	d := NewDispatcher(store, slog.Default())

	// Fill channel to capacity
	for i := 0; i < channelCap; i++ {
		d.Emit("board-1", EventCardCreated, nil)
	}

	// 1001st should not block
	done := make(chan bool, 1)
	go func() {
		d.Emit("board-1", EventCardCreated, nil)
		done <- true
	}()

	select {
	case <-done:
		// good
	case <-time.After(time.Second):
		t.Fatal("Emit blocked when channel is full")
	}
}

func TestDispatcherDelivery(t *testing.T) {
	ds := setupTestDB(t)
	boardID := createTestBoard(t, ds)
	store := NewStore(ds)
	logger := slog.Default()

	var received atomic.Int32
	var receivedSig string
	var receivedEvent string
	var receivedDeliveryID string
	var receivedBody []byte
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		mu.Lock()
		receivedSig = r.Header.Get("X-LWTS-Signature")
		receivedEvent = r.Header.Get("X-LWTS-Event")
		receivedDeliveryID = r.Header.Get("X-LWTS-Delivery")
		receivedBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ctx := context.Background()
	wh, _ := store.CreateWebhook(ctx, boardID, srv.URL, EventCardCreated)

	d := NewDispatcher(store, logger)
	d.Run()
	defer d.Stop()

	d.Emit(boardID, EventCardCreated, map[string]string{"title": "Test Card"})

	// Wait for delivery
	deadline := time.After(3 * time.Second)
	for {
		if received.Load() > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("delivery not received within 3s")
		case <-time.After(50 * time.Millisecond):
		}
	}

	mu.Lock()
	defer mu.Unlock()

	// Verify HMAC signature
	if !Verify(wh.Secret, receivedBody, receivedSig) {
		t.Error("HMAC signature verification failed")
	}

	// Verify headers
	if receivedEvent != EventCardCreated {
		t.Errorf("X-LWTS-Event = %q, want %q", receivedEvent, EventCardCreated)
	}
	if receivedDeliveryID == "" {
		t.Error("X-LWTS-Delivery header should be set")
	}

	// Verify payload envelope
	var payload Payload
	if err := json.Unmarshal(receivedBody, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Event != EventCardCreated {
		t.Errorf("payload.event = %q", payload.Event)
	}
	if payload.Board.ID != boardID {
		t.Errorf("payload.board.id = %q", payload.Board.ID)
	}
}

func TestDispatcherFanOut(t *testing.T) {
	ds := setupTestDB(t)
	boardID := createTestBoard(t, ds)
	store := NewStore(ds)

	var count atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ctx := context.Background()
	store.CreateWebhook(ctx, boardID, srv.URL+"/a", EventWildcard)
	store.CreateWebhook(ctx, boardID, srv.URL+"/b", EventCardCreated)

	d := NewDispatcher(store, slog.Default())
	d.Run()
	defer d.Stop()

	d.Emit(boardID, EventCardCreated, nil)

	deadline := time.After(3 * time.Second)
	for {
		if count.Load() >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("expected 2 deliveries, got %d", count.Load())
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestDispatcherTimeout(t *testing.T) {
	ds := setupTestDB(t)
	boardID := createTestBoard(t, ds)
	store := NewStore(ds)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second) // longer than 5s timeout
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ctx := context.Background()
	store.CreateWebhook(ctx, boardID, srv.URL, EventCardCreated)

	d := NewDispatcher(store, slog.Default())
	d.Run()
	defer d.Stop()

	d.Emit(boardID, EventCardCreated, nil)

	// Wait for the delivery to be processed (timeout should occur in ~5s)
	time.Sleep(7 * time.Second)

	// Check that delivery was scheduled for retry (attempt 0 → retry)
	deliveries, _ := store.ListDeliveries(ctx, "", 50)
	// List all deliveries regardless of webhook
	rows, _ := ds.Query(ctx, `SELECT status, attempts FROM webhook_deliveries LIMIT 10`)
	defer rows.Close()
	found := false
	for rows.Next() {
		var status string
		var attempts int
		rows.Scan(&status, &attempts)
		if attempts > 0 {
			found = true
		}
	}
	_ = deliveries
	if !found {
		t.Error("expected delivery to have attempts > 0 after timeout")
	}
}

func TestRetryAndFail(t *testing.T) {
	ds := setupTestDB(t)
	boardID := createTestBoard(t, ds)
	store := NewStore(ds)

	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(500)
		w.Write([]byte("Internal Server Error"))
	}))
	defer srv.Close()

	ctx := context.Background()
	wh, _ := store.CreateWebhook(ctx, boardID, srv.URL, EventCardCreated)

	// Create a delivery manually to test retry logic
	del, _ := store.CreateDelivery(ctx, wh.ID, EventCardCreated, `{"test":true}`)

	d := NewDispatcher(store, slog.Default())

	// Simulate deliver → should fail → schedule retry
	d.deliver(ctx, wh, del, []byte(`{"test":true}`))

	// Check it was marked for retry
	updated, _ := store.GetDelivery(ctx, del.ID)
	if updated.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", updated.Attempts)
	}
	if updated.NextRetryAt == nil {
		t.Fatal("next_retry_at should be set")
	}
	if updated.ResponseCode == nil || *updated.ResponseCode != 500 {
		t.Error("response_code should be 500")
	}

	// Second attempt
	del2, _ := store.GetDelivery(ctx, del.ID)
	d.deliver(ctx, wh, del2, []byte(`{"test":true}`))

	updated2, _ := store.GetDelivery(ctx, del.ID)
	if updated2.Attempts != 2 {
		t.Errorf("attempts = %d, want 2", updated2.Attempts)
	}

	// Third attempt → should mark as failed
	del3, _ := store.GetDelivery(ctx, del.ID)
	d.deliver(ctx, wh, del3, []byte(`{"test":true}`))

	final, _ := store.GetDelivery(ctx, del.ID)
	if final.Status != "failed" {
		t.Errorf("status = %q, want failed", final.Status)
	}
	if final.Attempts != 3 {
		t.Errorf("attempts = %d, want 3", final.Attempts)
	}

	// Response body should be recorded
	if final.ResponseBody == nil || *final.ResponseBody != "Internal Server Error" {
		t.Error("response_body should be recorded")
	}
}

// ── Handler ──

func TestHandlerCRUD(t *testing.T) {
	ds := setupTestDB(t)
	boardID := createTestBoard(t, ds)
	store := NewStore(ds)
	d := NewDispatcher(store, slog.Default())
	h := NewHandler(store, d)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Create
	body := `{"url":"https://example.com/hook","event_type":"card.created"}`
	req := httptest.NewRequest("POST", "/api/v1/boards/"+boardID+"/webhooks", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201, body: %s", w.Code, w.Body.String())
	}

	var created Webhook
	json.Unmarshal(w.Body.Bytes(), &created)
	if created.Secret == "" {
		t.Error("secret should be included in create response")
	}

	// List (secrets masked)
	req = httptest.NewRequest("GET", "/api/v1/boards/"+boardID+"/webhooks", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var listed []Webhook
	json.Unmarshal(w.Body.Bytes(), &listed)
	if len(listed) != 1 {
		t.Fatalf("listed %d, want 1", len(listed))
	}
	if listed[0].Secret == created.Secret {
		t.Error("secret should be masked in list response")
	}
	if !strings.Contains(listed[0].Secret, "****") {
		t.Errorf("masked secret = %q, should contain ****", listed[0].Secret)
	}

	// Get (secret masked)
	req = httptest.NewRequest("GET", "/api/v1/boards/"+boardID+"/webhooks/"+created.ID, nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var got Webhook
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.Secret == created.Secret {
		t.Error("secret should be masked in get response")
	}

	// Update: disable
	req = httptest.NewRequest("PATCH", "/api/v1/boards/"+boardID+"/webhooks/"+created.ID,
		strings.NewReader(`{"enabled":false}`))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var updated Webhook
	json.Unmarshal(w.Body.Bytes(), &updated)
	if updated.Enabled {
		t.Error("should be disabled")
	}

	// Delete
	req = httptest.NewRequest("DELETE", "/api/v1/boards/"+boardID+"/webhooks/"+created.ID, nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", w.Code)
	}

	// Verify gone
	req = httptest.NewRequest("GET", "/api/v1/boards/"+boardID+"/webhooks/"+created.ID, nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", w.Code)
	}
}

func TestHandlerValidation(t *testing.T) {
	ds := setupTestDB(t)
	boardID := createTestBoard(t, ds)
	store := NewStore(ds)
	d := NewDispatcher(store, slog.Default())
	h := NewHandler(store, d)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Missing fields
	req := httptest.NewRequest("POST", "/api/v1/boards/"+boardID+"/webhooks",
		strings.NewReader(`{"url":""}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("empty fields = %d, want 400", w.Code)
	}

	// Invalid URL scheme
	req = httptest.NewRequest("POST", "/api/v1/boards/"+boardID+"/webhooks",
		strings.NewReader(`{"url":"ftp://bad.com","event_type":"card.created"}`))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("ftp scheme = %d, want 400", w.Code)
	}
}

func TestHandlerDeliveries(t *testing.T) {
	ds := setupTestDB(t)
	boardID := createTestBoard(t, ds)
	store := NewStore(ds)
	d := NewDispatcher(store, slog.Default())
	h := NewHandler(store, d)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	ctx := context.Background()
	wh, _ := store.CreateWebhook(ctx, boardID, "https://example.com/hook", EventCardCreated)
	store.CreateDelivery(ctx, wh.ID, EventCardCreated, `{"test":1}`)
	store.CreateDelivery(ctx, wh.ID, EventCardCreated, `{"test":2}`)

	req := httptest.NewRequest("GET", "/api/v1/boards/"+boardID+"/webhooks/"+wh.ID+"/deliveries?limit=1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var deliveries []Delivery
	json.Unmarshal(w.Body.Bytes(), &deliveries)
	if len(deliveries) != 1 {
		t.Errorf("deliveries = %d, want 1 (limit=1)", len(deliveries))
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abcdefghijklmnop", "abcd********mnop"},
		{"short", "*****"},
		{"12345678", "********"},
		{"123456789", "1234*6789"},
	}
	for _, tt := range tests {
		got := maskSecret(tt.input)
		if got != tt.want {
			t.Errorf("maskSecret(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
