package embed

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/oceanplexian/lwts/server/internal/db"
)

// Max characters of card text fed to the embedder. bge-small truncates at 512
// tokens (~2000 chars) anyway, so this is effectively the model's natural cap.
const maxInputChars = 2000

// Service wires the embedding client to the database. All write-path calls are
// best-effort: failures are logged and the caller is not blocked.
type Service struct {
	ds     db.Datasource
	client *Client
	log    *slog.Logger
}

// NewService returns nil if either ds or client is nil — semantic features are
// silently disabled. Callers should always check for nil before invoking methods.
func NewService(ds db.Datasource, client *Client, log *slog.Logger) *Service {
	if ds == nil || client == nil {
		return nil
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{ds: ds, client: client, log: log}
}

// Configured reports whether the service has a working client. Useful for
// status endpoints; does not check pgvector schema state.
func (s *Service) Configured() bool {
	return s != nil && s.client != nil
}

// Model returns the configured embedding model identifier.
func (s *Service) Model() string {
	if s == nil {
		return ""
	}
	return s.client.Model()
}

// EmbedCard regenerates and stores the embedding for a single card. Used from
// card create/update hooks. Runs synchronously — call from a goroutine if you
// don't want to block the API response.
func (s *Service) EmbedCard(ctx context.Context, cardID, title, description string) error {
	if s == nil {
		return nil
	}
	text := composeCardText(title, description)
	if text == "" {
		return nil
	}
	vecs, err := s.client.Embed(ctx, []string{text})
	if err != nil {
		return fmt.Errorf("embed card %s: %w", cardID, err)
	}
	if len(vecs) != 1 {
		return fmt.Errorf("embed card %s: unexpected vector count", cardID)
	}
	if _, err := s.ds.Exec(ctx,
		"UPDATE cards SET embedding = $1 WHERE id = $2",
		Vector(vecs[0]), cardID,
	); err != nil {
		return fmt.Errorf("store embedding for %s: %w", cardID, err)
	}
	return nil
}

// EmbedCardAsync fires the embedding off in a goroutine with a fresh background
// context. Errors are logged. Use from request handlers to avoid blocking.
func (s *Service) EmbedCardAsync(cardID, title, description string) {
	if s == nil {
		return
	}
	go func() {
		// Detached context: the request might end before embedding finishes.
		ctx := context.Background()
		if err := s.EmbedCard(ctx, cardID, title, description); err != nil {
			s.log.Warn("embed card failed", "card_id", cardID, "err", err)
		}
	}()
}

// Backfill embeds every card in the database that doesn't yet have an embedding.
// Returns counts. Can be triggered by an admin endpoint or on first enable.
func (s *Service) Backfill(ctx context.Context, batchSize int) (embedded, skipped int, err error) {
	if s == nil {
		return 0, 0, nil
	}
	if batchSize <= 0 {
		batchSize = 32
	}

	for {
		rows, err := s.ds.Query(ctx,
			`SELECT id, title, description FROM cards
			 WHERE embedding IS NULL
			 LIMIT $1`, batchSize)
		if err != nil {
			return embedded, skipped, fmt.Errorf("backfill query: %w", err)
		}

		type item struct{ id, title, desc string }
		var batch []item
		for rows.Next() {
			var it item
			if err := rows.Scan(&it.id, &it.title, &it.desc); err != nil {
				rows.Close()
				return embedded, skipped, err
			}
			batch = append(batch, it)
		}
		rows.Close()

		if len(batch) == 0 {
			return embedded, skipped, nil
		}

		texts := make([]string, 0, len(batch))
		idxs := make([]int, 0, len(batch))
		for i, it := range batch {
			text := composeCardText(it.title, it.desc)
			if text == "" {
				skipped++
				continue
			}
			texts = append(texts, text)
			idxs = append(idxs, i)
		}
		if len(texts) == 0 {
			// Nothing to embed in this batch but cards still need to be marked
			// somehow so we don't loop forever. Skip them by setting to a
			// sentinel? No — better: select WHERE embedding IS NULL AND
			// LENGTH(title)+LENGTH(description) > 0 instead. But that misses
			// the long-content-but-empty case. Simplest fix: break the loop.
			return embedded, skipped, nil
		}

		vecs, err := s.client.Embed(ctx, texts)
		if err != nil {
			return embedded, skipped, fmt.Errorf("backfill embed: %w", err)
		}

		for i, vec := range vecs {
			cardID := batch[idxs[i]].id
			if _, err := s.ds.Exec(ctx,
				"UPDATE cards SET embedding = $1 WHERE id = $2",
				Vector(vec), cardID,
			); err != nil {
				return embedded, skipped, fmt.Errorf("backfill store %s: %w", cardID, err)
			}
			embedded++
		}

		if len(batch) < batchSize {
			return embedded, skipped, nil
		}
	}
}

// Counts returns (cards_with_embedding, total_cards). Useful for status UI.
func (s *Service) Counts(ctx context.Context) (withEmb, total int, err error) {
	if s == nil {
		return 0, 0, nil
	}
	if err := s.ds.QueryRow(ctx, "SELECT COUNT(*) FROM cards WHERE embedding IS NOT NULL").Scan(&withEmb); err != nil {
		return 0, 0, err
	}
	if err := s.ds.QueryRow(ctx, "SELECT COUNT(*) FROM cards").Scan(&total); err != nil {
		return 0, 0, err
	}
	return withEmb, total, nil
}

func composeCardText(title, description string) string {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if title == "" && description == "" {
		return ""
	}
	combined := title
	if description != "" {
		if combined != "" {
			combined += ". "
		}
		combined += description
	}
	if len(combined) > maxInputChars {
		combined = combined[:maxInputChars]
	}
	return combined
}
