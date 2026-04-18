//go:build integration
// +build integration

package embed

import (
	"context"
	"os"
	"testing"

	"github.com/oceanplexian/lwts/server/internal/db"
)

// pgDS opens a connection to the integration-test postgres. Skips if DB_URL is
// unset (mirrors the repo's other integration test conventions).
func pgDS(t *testing.T) db.Datasource {
	t.Helper()
	url := os.Getenv("DB_URL")
	if url == "" {
		t.Skip("DB_URL not set; skipping integration test")
	}
	ds, err := db.NewPostgresDatasource(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { ds.Close() })
	return ds
}

func TestEnsureSchema_PostgresWithPgvector(t *testing.T) {
	ds := pgDS(t)
	ctx := context.Background()

	// The integration postgres is shared across packages — prior test runs can
	// leave the `cards` table in various states. Reset to a known-empty table
	// so this test is self-contained regardless of execution order.
	if _, err := ds.Exec(ctx, "DROP TABLE IF EXISTS cards CASCADE"); err != nil {
		t.Fatalf("drop cards: %v", err)
	}
	if _, err := ds.Exec(ctx, "CREATE TABLE cards (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), title TEXT)"); err != nil {
		t.Fatalf("create cards: %v", err)
	}
	t.Cleanup(func() {
		// Drop ourselves so subsequent tests in the same package start clean and
		// the repo.Migrate() in those tests re-creates the full schema.
		_, _ = ds.Exec(context.Background(), "DROP TABLE IF EXISTS cards CASCADE")
	})

	if _, err := ds.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector"); err != nil {
		t.Skipf("pgvector not available, skipping: %v", err)
	}

	available, err := EnsureSchema(ctx, ds, 0)
	if err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	if !available {
		t.Fatal("expected available=true after enabling pgvector")
	}
	if !HasEmbeddingColumn(ctx, ds) {
		t.Fatal("expected embedding column after EnsureSchema")
	}

	// Idempotent.
	if _, err := EnsureSchema(ctx, ds, 0); err != nil {
		t.Fatalf("second EnsureSchema: %v", err)
	}
}
