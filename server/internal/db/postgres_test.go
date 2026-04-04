//go:build integration

package db

import (
	"context"
	"os"
	"testing"

	"github.com/oceanplexian/lwts/server/migrations"
)

func pgURL() string {
	if u := os.Getenv("DB_URL"); u != "" {
		return u
	}
	return "postgres://lwts_test:lwts_test@localhost:5433/lwts_test?sslmode=disable"
}

func newTestPG(t *testing.T) *PostgresDatasource {
	t.Helper()
	ctx := context.Background()
	ds, err := NewPostgresDatasource(ctx, pgURL())
	if err != nil {
		t.Fatalf("connect pg: %v", err)
	}
	t.Cleanup(func() {
		// Clean up all tables
		ds.Exec(ctx, "DROP SCHEMA public CASCADE")
		ds.Exec(ctx, "CREATE SCHEMA public")
		ds.Close()
	})
	// Clean slate
	ds.Exec(ctx, "DROP SCHEMA public CASCADE")
	ds.Exec(ctx, "CREATE SCHEMA public")
	return ds
}

func TestPostgresPing(t *testing.T) {
	ds := newTestPG(t)
	if err := ds.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestPostgresDBType(t *testing.T) {
	ds := newTestPG(t)
	if ds.DBType() != "postgres" {
		t.Fatalf("expected postgres, got %s", ds.DBType())
	}
}

func TestPostgresSelectOne(t *testing.T) {
	ds := newTestPG(t)
	var n int
	if err := ds.QueryRow(context.Background(), "SELECT 1").Scan(&n); err != nil {
		t.Fatalf("select 1: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1, got %d", n)
	}
}

func TestPostgresTransaction(t *testing.T) {
	ds := newTestPG(t)
	ctx := context.Background()
	ds.Exec(ctx, "CREATE TABLE txtest (val TEXT)")

	tx, _ := ds.Begin(ctx)
	tx.Exec(ctx, "INSERT INTO txtest (val) VALUES ($1)", "yes")
	tx.Commit(ctx)

	var val string
	ds.QueryRow(ctx, "SELECT val FROM txtest").Scan(&val)
	if val != "yes" {
		t.Fatalf("expected yes, got %s", val)
	}

	tx2, _ := ds.Begin(ctx)
	tx2.Exec(ctx, "INSERT INTO txtest (val) VALUES ($1)", "no")
	tx2.Rollback(ctx)

	var count int
	ds.QueryRow(ctx, "SELECT COUNT(*) FROM txtest").Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1, got %d", count)
	}
}

func TestPostgresMigrateAll(t *testing.T) {
	ds := newTestPG(t)
	ctx := context.Background()

	if err := Migrate(ctx, ds, migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Verify all tables
	tables := []string{"users", "refresh_tokens", "boards", "cards", "comments", "webhooks", "webhook_deliveries", "invites"}
	for _, table := range tables {
		var exists bool
		err := ds.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name=$1)", table).Scan(&exists)
		if err != nil || !exists {
			t.Errorf("table %s not found", table)
		}
	}

	var count int
	ds.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if count != 7 {
		t.Fatalf("expected 7 migrations, got %d", count)
	}
}

func TestPostgresMigrateIdempotent(t *testing.T) {
	ds := newTestPG(t)
	ctx := context.Background()

	Migrate(ctx, ds, migrations.FS)
	if err := Migrate(ctx, ds, migrations.FS); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	var count int
	ds.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if count != 7 {
		t.Fatalf("expected 7, got %d", count)
	}
}

func TestPostgresGenRandomUUID(t *testing.T) {
	ds := newTestPG(t)
	ctx := context.Background()
	Migrate(ctx, ds, migrations.FS)

	// Insert without specifying ID — should auto-generate UUID
	var id string
	err := ds.QueryRow(ctx,
		"INSERT INTO users (email, name, password_hash, initials) VALUES ($1, $2, $3, $4) RETURNING id",
		"auto@test.com", "Auto", "hash", "AU",
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if len(id) != 36 {
		t.Fatalf("expected UUID, got %s", id)
	}
}

func TestPostgresCascade(t *testing.T) {
	ds := newTestPG(t)
	ctx := context.Background()
	Migrate(ctx, ds, migrations.FS)

	var uid, bid, cid string
	ds.QueryRow(ctx, "INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id", "x@y.com", "X", "h").Scan(&uid)
	ds.QueryRow(ctx, "INSERT INTO boards (name, owner_id) VALUES ($1, $2) RETURNING id", "B", uid).Scan(&bid)
	ds.QueryRow(ctx, "INSERT INTO cards (board_id, column_id, title, key) VALUES ($1, $2, $3, $4) RETURNING id", bid, "todo", "C", "K-1").Scan(&cid)
	ds.Exec(ctx, "INSERT INTO comments (card_id, author_id, body) VALUES ($1, $2, $3)", cid, uid, "hi")

	ds.Exec(ctx, "DELETE FROM boards WHERE id = $1", bid)

	var cardCount, commentCount int
	ds.QueryRow(ctx, "SELECT COUNT(*) FROM cards").Scan(&cardCount)
	ds.QueryRow(ctx, "SELECT COUNT(*) FROM comments").Scan(&commentCount)
	if cardCount != 0 || commentCount != 0 {
		t.Fatalf("cascade failed: cards=%d comments=%d", cardCount, commentCount)
	}
}

func TestPostgresCheckConstraint(t *testing.T) {
	ds := newTestPG(t)
	ctx := context.Background()
	Migrate(ctx, ds, migrations.FS)

	var uid, bid string
	ds.QueryRow(ctx, "INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id", "z@z.com", "Z", "h").Scan(&uid)
	ds.QueryRow(ctx, "INSERT INTO boards (name, owner_id) VALUES ($1, $2) RETURNING id", "B", uid).Scan(&bid)

	_, err := ds.Exec(ctx,
		"INSERT INTO cards (board_id, column_id, title, key, priority) VALUES ($1, $2, $3, $4, $5)",
		bid, "todo", "C", "K-2", "invalid",
	)
	if err == nil {
		t.Fatal("expected check constraint error")
	}
}
