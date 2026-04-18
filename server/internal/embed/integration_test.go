//go:build integration
// +build integration

package embed

import (
	"context"
	"os"
	"testing"

	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/oceanplexian/lwts/server/migrations"
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

	// We need the cards table to exist before EnsureSchema can ALTER it.
	if err := db.Migrate(ctx, ds, migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Pgvector must be installable. The CI service image ships it.
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
