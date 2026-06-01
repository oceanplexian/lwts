//go:build integration

package repo

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/oceanplexian/lwts/server/migrations"
)

// TestCardImageRoundTripIntegration exercises the card_images repo against the
// configured backend (Postgres in CI's integration job, SQLite otherwise),
// proving BYTEA/BLOB bytes round-trip identically on the real prod backend.
func TestCardImageRoundTripIntegration(t *testing.T) {
	ctx := context.Background()
	url := os.Getenv("DB_URL")
	if url == "" {
		url = "postgres://lwts_test:lwts_test@localhost:5433/lwts_test?sslmode=disable"
	}

	ds, err := db.NewDatasource(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(url, "postgres") {
			_, _ = ds.Exec(ctx, "DROP SCHEMA public CASCADE")
			_, _ = ds.Exec(ctx, "CREATE SCHEMA public")
		}
		ds.Close()
	})

	if err := db.Migrate(ctx, ds, migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	users := NewUserRepository(ds)
	boards := NewBoardRepository(ds)
	cards := NewCardRepository(ds)
	images := NewCardImageRepository(ds)

	user, err := users.Create(ctx, "U", "u@example.com", "h")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	board, err := boards.Create(ctx, "B", "LWTS", user.ID)
	if err != nil {
		t.Fatalf("create board: %v", err)
	}
	card, err := cards.Create(ctx, board.ID, CardCreate{ColumnID: "todo", Title: "img"})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	// Bytes that include a NUL and high bytes — a strict round-trip check.
	payload := []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0xff, 0x10, 0x42}
	img, err := images.Create(ctx, card.ID, "x.png", "image/png", payload, &user.ID)
	if err != nil {
		t.Fatalf("create image: %v", err)
	}
	if img.SizeBytes != len(payload) {
		t.Errorf("size = %d, want %d", img.SizeBytes, len(payload))
	}

	gotMeta, gotData, err := images.GetData(ctx, img.ID)
	if err != nil {
		t.Fatalf("get data: %v", err)
	}
	if !bytes.Equal(gotData, payload) {
		t.Errorf("bytes differ: got %x want %x", gotData, payload)
	}
	if gotMeta.CardID != card.ID || gotMeta.ContentType != "image/png" {
		t.Errorf("meta mismatch: %+v", gotMeta)
	}
	if gotMeta.UploadedBy == nil || *gotMeta.UploadedBy != user.ID {
		t.Errorf("uploaded_by = %v, want %s", gotMeta.UploadedBy, user.ID)
	}

	listed, err := images.ListByCard(ctx, card.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("list len = %d, want 1", len(listed))
	}

	if err := images.Delete(ctx, img.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := images.GetMeta(ctx, img.ID); err != ErrNotFound {
		t.Errorf("after delete err = %v, want ErrNotFound", err)
	}
}
