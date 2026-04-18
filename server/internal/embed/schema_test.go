package embed

import (
	"context"
	"testing"

	"github.com/oceanplexian/lwts/server/internal/db"
)

func TestEnsureSchema_SQLiteIsNoop(t *testing.T) {
	ds, err := db.NewSQLiteDatasource("sqlite://:memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer ds.Close()

	available, err := EnsureSchema(context.Background(), ds, 0)
	if err != nil {
		t.Fatalf("expected no error on sqlite, got %v", err)
	}
	if available {
		t.Fatal("expected unavailable on sqlite")
	}
}

func TestHasEmbeddingColumn_SQLite(t *testing.T) {
	ds, err := db.NewSQLiteDatasource("sqlite://:memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer ds.Close()

	if HasEmbeddingColumn(context.Background(), ds) {
		t.Fatal("expected false on sqlite")
	}
}

func TestEscapeRegex(t *testing.T) {
	cases := map[string]string{
		"hello":       "hello",
		"foo-bar":     `foo\-bar`,
		"a.b":         `a\.b`,
		"x*y":         `x\*y`,
		"hello world": `hello\ world`,
	}
	for in, want := range cases {
		if got := escapeRegex(in); got != want {
			t.Errorf("escapeRegex(%q) = %q, want %q", in, got, want)
		}
	}
}
