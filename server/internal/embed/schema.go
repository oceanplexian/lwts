package embed

import (
	"context"
	"fmt"

	"github.com/oceanplexian/lwts/server/internal/db"
)

// Default embedding dimension. bge-small-en-v1.5 is 384.
// Override via EMBEDDING_DIM env var if using a different model
// (e.g. text-embedding-3-small needs 1536).
const DefaultDim = 384

// EnsureSchema provisions the cards.embedding column and HNSW index
// IF the database is postgres AND the pgvector extension is available.
//
// Returns:
//   - available=true if pgvector is present and the schema is now ready
//   - available=false (no error) if pgvector is missing — caller should treat
//     semantic search as unavailable but the app should keep running
//   - err for unexpected DB failures
//
// This is called once at startup, after migrations.
func EnsureSchema(ctx context.Context, ds db.Datasource, dim int) (available bool, err error) {
	if ds.DBType() != "postgres" {
		return false, nil
	}
	if dim <= 0 {
		dim = DefaultDim
	}

	// Probe for the vector extension.
	var present bool
	err = ds.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector')").Scan(&present)
	if err != nil {
		return false, fmt.Errorf("probe pg_extension: %w", err)
	}
	if !present {
		return false, nil
	}

	// Create column. IF NOT EXISTS keeps this idempotent across restarts.
	colSQL := fmt.Sprintf("ALTER TABLE cards ADD COLUMN IF NOT EXISTS embedding vector(%d)", dim)
	if _, err := ds.Exec(ctx, colSQL); err != nil {
		return false, fmt.Errorf("add embedding column: %w", err)
	}

	// HNSW for cosine distance. Single index, sized for current corpus.
	idxSQL := "CREATE INDEX IF NOT EXISTS idx_cards_embedding ON cards USING hnsw (embedding vector_cosine_ops)"
	if _, err := ds.Exec(ctx, idxSQL); err != nil {
		return false, fmt.Errorf("create hnsw index: %w", err)
	}

	return true, nil
}

// HasEmbeddingColumn returns true if the cards table currently has an embedding
// column. Used by the search dispatcher to decide whether semantic search can run.
func HasEmbeddingColumn(ctx context.Context, ds db.Datasource) bool {
	if ds.DBType() != "postgres" {
		return false
	}
	var present bool
	err := ds.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM information_schema.columns
		   WHERE table_name = 'cards' AND column_name = 'embedding'
		 )`).Scan(&present)
	if err != nil {
		return false
	}
	return present
}
