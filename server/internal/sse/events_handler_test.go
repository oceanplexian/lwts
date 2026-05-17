package sse

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/oceanplexian/lwts/server/internal/auth"
	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/oceanplexian/lwts/server/internal/repo"
)

// seedUser creates a user row and returns (id, email).
func seedUser(t *testing.T, ds db.Datasource) (string, string) {
	t.Helper()
	id := uuid.NewString()
	email := "u-" + uuid.NewString() + "@test.com"
	if _, err := ds.Exec(context.Background(),
		`INSERT INTO users (id, email, name, password_hash) VALUES ($1, $2, $3, $4)`,
		id, email, "Tester", "hash"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id, email
}

// seedAPIKey writes an lwts_sk_ key to api_keys and returns the raw token.
func seedAPIKey(t *testing.T, ds db.Datasource, userID string) string {
	t.Helper()
	raw := "lwts_sk_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	hash := sha256.Sum256([]byte(raw))
	if _, err := ds.Exec(context.Background(),
		`INSERT INTO api_keys (id, user_id, name, key_hash, key_prefix, key_full, permissions, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		uuid.NewString(), userID, "test", hex.EncodeToString(hash[:]),
		raw[:12], raw, "[]", time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("seed api key: %v", err)
	}
	return raw
}

type stubUserStore struct{ ds db.Datasource }

func (s *stubUserStore) GetUserByID(ctx context.Context, id string) (*repo.User, error) {
	var u repo.User
	err := s.ds.QueryRow(ctx, `SELECT id, email, name FROM users WHERE id = $1`, id).Scan(&u.ID, &u.Email, &u.Name)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
func (s *stubUserStore) CreateUser(context.Context, string, string, string, string, string, string) (*repo.User, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *stubUserStore) GetUserByEmail(context.Context, string) (*repo.User, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *stubUserStore) CountUsers(context.Context) (int, error) { return 0, nil }
func (s *stubUserStore) UpdateUserRole(context.Context, string, string) error {
	return fmt.Errorf("not implemented")
}
func (s *stubUserStore) UpdateUser(context.Context, string, repo.UserUpdate) (*repo.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestEventsHandler_RequiresAuth(t *testing.T) {
	ds := setupTestDB(t)
	hub := startHub(t)
	users := &stubUserStore{ds: ds}
	handler := EventsHandler(hub, NewDBEventStore(ds), "secret", ds, users)

	req := httptest.NewRequest("GET", "/api/v1/boards/b1/events", nil)
	req.SetPathValue("id", "b1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestEventsHandler_AcceptsAPIKey(t *testing.T) {
	ds := setupTestDB(t)
	boardID := seedBoard(t, ds)
	userID, _ := seedUser(t, ds)
	apiKey := seedAPIKey(t, ds, userID)

	store := NewDBEventStore(ds)
	hub := NewHub()
	hub.SetStore(store)
	go hub.Run()
	t.Cleanup(hub.Stop)

	users := &stubUserStore{ds: ds}
	handler := EventsHandler(hub, store, "secret", ds, users)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest("GET", "/api/v1/boards/"+boardID+"/events?token="+apiKey, nil).WithContext(ctx)
	req.SetPathValue("id", boardID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("expected SSE response, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "event: connected") {
		t.Fatalf("expected connected event, got: %s", w.Body.String())
	}
}

func TestEventsHandler_ReplaysFromLastEventID(t *testing.T) {
	ds := setupTestDB(t)
	boardID := seedBoard(t, ds)
	userID, email := seedUser(t, ds)

	store := NewDBEventStore(ds)
	// Pre-seed three events so we can resume from id 1 and expect ids 2 & 3.
	bctx := context.Background()
	e1, err := store.Persist(bctx, boardID, "card_created", []byte(`{"id":"a","title":"alpha"}`), "")
	if err != nil {
		t.Fatalf("persist e1: %v", err)
	}
	e2, err := store.Persist(bctx, boardID, "card_updated", []byte(`{"id":"a","title":"beta"}`), "")
	if err != nil {
		t.Fatalf("persist e2: %v", err)
	}
	e3, err := store.Persist(bctx, boardID, "card_moved", []byte(`{"id":"a","from_column_id":"backlog","to_column_id":"todo"}`), "")
	if err != nil {
		t.Fatalf("persist e3: %v", err)
	}
	_ = e1
	_ = e2
	_ = e3

	hub := NewHub()
	hub.SetStore(store)
	go hub.Run()
	t.Cleanup(hub.Stop)

	users := &stubUserStore{ds: ds}
	pair, _, err := auth.IssueTokens("secret", userID, email, "member")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	handler := EventsHandler(hub, store, "secret", ds, users)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	req := httptest.NewRequest("GET", "/api/v1/boards/"+boardID+"/events", nil).WithContext(ctx)
	req.SetPathValue("id", boardID)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	req.Header.Set("Last-Event-ID", strconv.FormatInt(e1.ID, 10))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Parse the SSE body for id: lines, expecting only e2 and e3 (not e1).
	var seenIDs []int64
	sc := bufio.NewScanner(bytes.NewReader(w.Body.Bytes()))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "id: ") {
			n, _ := strconv.ParseInt(strings.TrimPrefix(line, "id: "), 10, 64)
			seenIDs = append(seenIDs, n)
		}
	}
	if len(seenIDs) != 2 || seenIDs[0] != e2.ID || seenIDs[1] != e3.ID {
		t.Fatalf("expected replay of [%d %d], got %v", e2.ID, e3.ID, seenIDs)
	}
}

func TestEventsHandler_ConnectedAdvertisesLastEventID(t *testing.T) {
	ds := setupTestDB(t)
	boardID := seedBoard(t, ds)
	userID, email := seedUser(t, ds)

	store := NewDBEventStore(ds)
	if _, err := store.Persist(context.Background(), boardID, "card_created", []byte(`{}`), ""); err != nil {
		t.Fatalf("persist: %v", err)
	}

	hub := NewHub()
	hub.SetStore(store)
	go hub.Run()
	t.Cleanup(hub.Stop)

	users := &stubUserStore{ds: ds}
	pair, _, _ := auth.IssueTokens("secret", userID, email, "member")
	handler := EventsHandler(hub, store, "secret", ds, users)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest("GET", "/api/v1/boards/"+boardID+"/events", nil).WithContext(ctx)
	req.SetPathValue("id", boardID)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()
	// Connected event payload contains last_event_id with the high-water mark.
	// We just check the field name is present and parsable.
	idx := strings.Index(body, `"last_event_id":`)
	if idx == -1 {
		t.Fatalf("connected payload missing last_event_id; body=%s", body)
	}
	// Pull out the next line "data: {...}" and parse JSON.
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var payload map[string]any
		if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &payload) == nil {
			if _, ok := payload["last_event_id"]; ok {
				return // pass
			}
		}
	}
	t.Fatalf("did not find parsable connected payload; body=%s", body)
}
