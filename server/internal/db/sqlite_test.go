package db

import (
	"context"
	"testing"

	"github.com/oceanplexian/lwts/server/migrations"
)

func newTestSQLite(t *testing.T) *SQLiteDatasource {
	t.Helper()
	ds, err := NewSQLiteDatasource("sqlite://:memory:")
	if err != nil {
		t.Fatalf("create sqlite: %v", err)
	}
	t.Cleanup(func() { ds.Close() })
	return ds
}

func TestSQLitePing(t *testing.T) {
	ds := newTestSQLite(t)
	if err := ds.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestSQLiteDBType(t *testing.T) {
	ds := newTestSQLite(t)
	if ds.DBType() != "sqlite" {
		t.Fatalf("expected sqlite, got %s", ds.DBType())
	}
}

func TestSQLiteSelectOne(t *testing.T) {
	ds := newTestSQLite(t)
	var n int
	if err := ds.QueryRow(context.Background(), "SELECT 1").Scan(&n); err != nil {
		t.Fatalf("select 1: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1, got %d", n)
	}
}

func TestSQLitePlaceholderConversion(t *testing.T) {
	ds := newTestSQLite(t)
	ctx := context.Background()
	_, _ = ds.Exec(ctx, "CREATE TABLE test (a TEXT, b TEXT)")
	_, _ = ds.Exec(ctx, "INSERT INTO test (a, b) VALUES ($1, $2)", "hello", "world")

	var a, b string
	if err := ds.QueryRow(ctx, "SELECT a, b FROM test WHERE a = $1", "hello").Scan(&a, &b); err != nil {
		t.Fatalf("query: %v", err)
	}
	if a != "hello" || b != "world" {
		t.Fatalf("got %s, %s", a, b)
	}
}

func TestSQLiteWALMode(t *testing.T) {
	ds := newTestSQLite(t)
	var mode string
	if err := ds.QueryRow(context.Background(), "PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("pragma: %v", err)
	}
	// In-memory databases use "memory" journal mode, WAL is for file-backed
	if mode != "wal" && mode != "memory" {
		t.Fatalf("expected wal or memory, got %s", mode)
	}
}

func TestSQLiteForeignKeys(t *testing.T) {
	ds := newTestSQLite(t)
	ctx := context.Background()
	_, _ = ds.Exec(ctx, "CREATE TABLE parent (id INTEGER PRIMARY KEY)")
	_, _ = ds.Exec(ctx, "CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES parent(id))")

	_, err := ds.Exec(ctx, "INSERT INTO child (id, parent_id) VALUES (1, 999)")
	if err == nil {
		t.Fatal("expected foreign key error, got nil")
	}
}

func TestSQLiteTransaction(t *testing.T) {
	ds := newTestSQLite(t)
	ctx := context.Background()
	_, _ = ds.Exec(ctx, "CREATE TABLE txtest (val TEXT)")

	tx, err := ds.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	_, _ = tx.Exec(ctx, "INSERT INTO txtest (val) VALUES ($1)", "committed")
	_ = tx.Commit(ctx)

	var val string
	_ = ds.QueryRow(ctx, "SELECT val FROM txtest").Scan(&val)
	if val != "committed" {
		t.Fatalf("expected committed, got %s", val)
	}

	// Test rollback
	tx2, _ := ds.Begin(ctx)
	_, _ = tx2.Exec(ctx, "INSERT INTO txtest (val) VALUES ($1)", "rolled-back")
	_ = tx2.Rollback(ctx)

	var count int
	_ = ds.QueryRow(ctx, "SELECT COUNT(*) FROM txtest").Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 row after rollback, got %d", count)
	}
}

func TestSQLiteNoRows(t *testing.T) {
	ds := newTestSQLite(t)
	ctx := context.Background()
	_, _ = ds.Exec(ctx, "CREATE TABLE empty (id INTEGER)")

	var id int
	err := ds.QueryRow(ctx, "SELECT id FROM empty WHERE id = 1").Scan(&id)
	if err != ErrNoRows {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
}

func TestSQLiteMigrateAll(t *testing.T) {
	ds := newTestSQLite(t)
	ctx := context.Background()

	if err := Migrate(ctx, ds, migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Verify tables exist
	tables := []string{"users", "refresh_tokens", "boards", "cards", "comments", "webhooks", "webhook_deliveries", "invites"}
	for _, table := range tables {
		var name string
		err := ds.QueryRow(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name=$1", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}

	// Verify schema_migrations
	var count int
	_ = ds.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if count != 16 {
		t.Fatalf("expected 16 migrations, got %d", count)
	}
}

func TestSQLiteMigrateIdempotent(t *testing.T) {
	ds := newTestSQLite(t)
	ctx := context.Background()

	if err := Migrate(ctx, ds, migrations.FS); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(ctx, ds, migrations.FS); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	var count int
	_ = ds.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if count != 16 {
		t.Fatalf("expected 16 migrations after idempotent run, got %d", count)
	}
}

func TestSQLiteInsertAndQueryUser(t *testing.T) {
	ds := newTestSQLite(t)
	ctx := context.Background()
	_ = Migrate(ctx, ds, migrations.FS)

	id := "550e8400-e29b-41d4-a716-446655440000"
	_, err := ds.Exec(ctx,
		"INSERT INTO users (id, email, name, password_hash, initials) VALUES ($1, $2, $3, $4, $5)",
		id, "test@example.com", "Test User", "hash123", "TU",
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var email, name string
	err = ds.QueryRow(ctx, "SELECT email, name FROM users WHERE id = $1", id).Scan(&email, &name)
	if err != nil {
		t.Fatalf("query user: %v", err)
	}
	if email != "test@example.com" || name != "Test User" {
		t.Fatalf("got %s, %s", email, name)
	}
}

func TestSQLiteCascadeDelete(t *testing.T) {
	ds := newTestSQLite(t)
	ctx := context.Background()
	_ = Migrate(ctx, ds, migrations.FS)

	// Create user -> board -> card -> comment, then delete board
	uid := "550e8400-e29b-41d4-a716-446655440001"
	bid := "550e8400-e29b-41d4-a716-446655440002"
	cid := "550e8400-e29b-41d4-a716-446655440003"

	_, _ = ds.Exec(ctx, "INSERT INTO users (id, email, name, password_hash) VALUES ($1, $2, $3, $4)", uid, "a@b.com", "A", "h")
	_, _ = ds.Exec(ctx, "INSERT INTO boards (id, name, owner_id) VALUES ($1, $2, $3)", bid, "Board", uid)
	_, _ = ds.Exec(ctx, "INSERT INTO cards (id, board_id, column_id, title, key) VALUES ($1, $2, $3, $4, $5)", cid, bid, "todo", "Card", "LWTS-1")
	_, _ = ds.Exec(ctx, "INSERT INTO comments (id, card_id, author_id, body) VALUES ($1, $2, $3, $4)", "550e8400-e29b-41d4-a716-446655440004", cid, uid, "hello")

	_, _ = ds.Exec(ctx, "DELETE FROM boards WHERE id = $1", bid)

	var cardCount, commentCount int
	_ = ds.QueryRow(ctx, "SELECT COUNT(*) FROM cards").Scan(&cardCount)
	_ = ds.QueryRow(ctx, "SELECT COUNT(*) FROM comments").Scan(&commentCount)
	if cardCount != 0 || commentCount != 0 {
		t.Fatalf("cascade failed: cards=%d, comments=%d", cardCount, commentCount)
	}
}

func TestSQLiteCheckConstraint(t *testing.T) {
	ds := newTestSQLite(t)
	ctx := context.Background()
	_ = Migrate(ctx, ds, migrations.FS)

	uid := "550e8400-e29b-41d4-a716-446655440010"
	bid := "550e8400-e29b-41d4-a716-446655440011"
	_, _ = ds.Exec(ctx, "INSERT INTO users (id, email, name, password_hash) VALUES ($1, $2, $3, $4)", uid, "c@d.com", "C", "h")
	_, _ = ds.Exec(ctx, "INSERT INTO boards (id, name, owner_id) VALUES ($1, $2, $3)", bid, "Board", uid)

	_, err := ds.Exec(ctx,
		"INSERT INTO cards (id, board_id, column_id, title, key, priority) VALUES ($1, $2, $3, $4, $5, $6)",
		"550e8400-e29b-41d4-a716-446655440012", bid, "todo", "Card", "LWTS-2", "invalid",
	)
	if err == nil {
		t.Fatal("expected check constraint error for invalid priority")
	}
}
