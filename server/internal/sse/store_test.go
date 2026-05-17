package sse

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/oceanplexian/lwts/server/migrations"
)

func setupTestDB(t *testing.T) db.Datasource {
	t.Helper()
	ds, err := db.NewSQLiteDatasource("sqlite://:memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Migrate(context.Background(), ds, migrations.FS); err != nil {
		ds.Close()
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { ds.Close() })
	return ds
}

// seedBoard creates a board (and its owner user) and returns the board id.
// board_events has an FK to boards, so we need a real board to insert against.
//
// SQLite drops the gen_random_uuid() defaults, so we generate ids in code.
func seedBoard(t *testing.T, ds db.Datasource) string {
	t.Helper()
	ctx := context.Background()
	ownerID := uuid.NewString()
	email := "owner-" + uuid.NewString() + "@test.com"
	if _, err := ds.Exec(ctx,
		`INSERT INTO users (id, email, name, password_hash) VALUES ($1, $2, $3, $4)`,
		ownerID, email, "Owner", "hash"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	boardID := uuid.NewString()
	if _, err := ds.Exec(ctx,
		`INSERT INTO boards (id, name, owner_id) VALUES ($1, $2, $3)`,
		boardID, "Test Board", ownerID); err != nil {
		t.Fatalf("insert board: %v", err)
	}
	return boardID
}

func TestDBEventStore_PersistAssignsMonotonicID(t *testing.T) {
	ds := setupTestDB(t)
	store := NewDBEventStore(ds)
	boardID := seedBoard(t, ds)
	ctx := context.Background()

	e1, err := store.Persist(ctx, boardID, "card_created", []byte(`{"id":"a"}`), "user-1")
	if err != nil {
		t.Fatalf("persist 1: %v", err)
	}
	e2, err := store.Persist(ctx, boardID, "card_moved", []byte(`{"id":"a","from_column_id":"backlog"}`), "user-1")
	if err != nil {
		t.Fatalf("persist 2: %v", err)
	}
	if e2.ID <= e1.ID {
		t.Fatalf("expected monotonic ids, got %d then %d", e1.ID, e2.ID)
	}
	if e1.EventType != "card_created" {
		t.Fatalf("unexpected event type: %s", e1.EventType)
	}
}

func TestDBEventStore_PersistRejectsBadJSON(t *testing.T) {
	ds := setupTestDB(t)
	store := NewDBEventStore(ds)
	boardID := seedBoard(t, ds)
	ctx := context.Background()

	if _, err := store.Persist(ctx, boardID, "card_created", []byte(`{not json`), "u"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDBEventStore_LoadSinceFiltersBoardAndID(t *testing.T) {
	ds := setupTestDB(t)
	store := NewDBEventStore(ds)
	boardA := seedBoard(t, ds)
	boardB := seedBoard(t, ds)
	ctx := context.Background()

	var firstA, secondA StoredEvent
	var err error
	firstA, err = store.Persist(ctx, boardA, "card_created", []byte(`{"n":1}`), "")
	if err != nil {
		t.Fatalf("persist A1: %v", err)
	}
	secondA, err = store.Persist(ctx, boardA, "card_updated", []byte(`{"n":2}`), "")
	if err != nil {
		t.Fatalf("persist A2: %v", err)
	}
	if _, err := store.Persist(ctx, boardB, "card_created", []byte(`{"n":1}`), ""); err != nil {
		t.Fatalf("persist B1: %v", err)
	}

	got, err := store.LoadSince(ctx, boardA, firstA.ID-1, 100)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 2 || got[0].ID != firstA.ID || got[1].ID != secondA.ID {
		t.Fatalf("expected both board A events in order, got %+v", got)
	}

	got, err = store.LoadSince(ctx, boardA, firstA.ID, 100)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 || got[0].ID != secondA.ID {
		t.Fatalf("expected only second event after cursor, got %+v", got)
	}
}

func TestDBEventStore_MaxIDReturnsZeroWhenEmpty(t *testing.T) {
	ds := setupTestDB(t)
	store := NewDBEventStore(ds)
	boardID := seedBoard(t, ds)
	ctx := context.Background()

	max, err := store.MaxID(ctx, boardID)
	if err != nil {
		t.Fatalf("max: %v", err)
	}
	if max != 0 {
		t.Fatalf("expected 0, got %d", max)
	}

	if _, err := store.Persist(ctx, boardID, "card_created", []byte(`{}`), ""); err != nil {
		t.Fatalf("persist: %v", err)
	}
	max, err = store.MaxID(ctx, boardID)
	if err != nil {
		t.Fatalf("max: %v", err)
	}
	if max == 0 {
		t.Fatal("expected non-zero max after insert")
	}
}

func TestDBEventStore_PurgeOlderThan(t *testing.T) {
	ds := setupTestDB(t)
	store := NewDBEventStore(ds)
	boardID := seedBoard(t, ds)
	ctx := context.Background()

	if _, err := store.Persist(ctx, boardID, "card_created", []byte(`{}`), ""); err != nil {
		t.Fatalf("persist: %v", err)
	}

	// Cutoff in the future deletes everything.
	deleted, err := store.PurgeOlderThan(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted, got %d", deleted)
	}

	max, _ := store.MaxID(ctx, boardID)
	if max != 0 {
		t.Fatalf("expected empty after purge, got max=%d", max)
	}
}
