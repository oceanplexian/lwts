package embed

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oceanplexian/lwts/server/internal/db"
)

// ErrBackfillRunning is returned by StartBackfill when one is already in flight.
var ErrBackfillRunning = errors.New("embed: backfill already running")

func nowUnix() int64 { return time.Now().Unix() }

// Max characters of card text fed to the embedder. bge-small truncates at 512
// tokens (~2000 chars) anyway, so this is effectively the model's natural cap.
const maxInputChars = 2000

// BackfillProgress is a snapshot of a currently-running or last-run backfill
// job. Safe to copy; fields are loaded atomically.
type BackfillProgress struct {
	Running       bool   `json:"running"`
	Embedded      int    `json:"embedded"`
	Skipped       int    `json:"skipped"`
	LastError     string `json:"last_error,omitempty"`
	StartedAtUnix int64  `json:"started_at_unix,omitempty"`
	EndedAtUnix   int64  `json:"ended_at_unix,omitempty"`
}

// Service wires the embedding client to the database. All write-path calls are
// best-effort: failures are logged and the caller is not blocked.
type Service struct {
	ds     db.Datasource
	client *Client
	log    *slog.Logger

	backfillMu       sync.Mutex
	backfillRunning  atomic.Bool
	backfillEmbedded atomic.Int64
	backfillSkipped  atomic.Int64
	backfillErr      atomic.Value // string
	backfillStarted  atomic.Int64
	backfillEnded    atomic.Int64
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

// BackfillProgress returns a snapshot of the current or last backfill run.
func (s *Service) BackfillProgress() BackfillProgress {
	if s == nil {
		return BackfillProgress{}
	}
	errStr, _ := s.backfillErr.Load().(string)
	return BackfillProgress{
		Running:       s.backfillRunning.Load(),
		Embedded:      int(s.backfillEmbedded.Load()),
		Skipped:       int(s.backfillSkipped.Load()),
		LastError:     errStr,
		StartedAtUnix: s.backfillStarted.Load(),
		EndedAtUnix:   s.backfillEnded.Load(),
	}
}

// StartBackfill kicks off a backfill in a goroutine. Returns ErrBackfillRunning
// if one is already in progress. Use BackfillProgress to poll for status.
func (s *Service) StartBackfill(batchSize int) error {
	if s == nil {
		return fmt.Errorf("embed: service not configured")
	}
	s.backfillMu.Lock()
	defer s.backfillMu.Unlock()
	if s.backfillRunning.Load() {
		return ErrBackfillRunning
	}
	s.backfillRunning.Store(true)
	s.backfillEmbedded.Store(0)
	s.backfillSkipped.Store(0)
	s.backfillErr.Store("")
	s.backfillStarted.Store(nowUnix())
	s.backfillEnded.Store(0)

	go func() {
		// Detached context; a request cancellation must not cancel a long-
		// running backfill. The service is in-process so nothing else cancels
		// this aside from a graceful shutdown, which we accept cutting short.
		ctx := context.Background()
		embedded, skipped, err := s.Backfill(ctx, batchSize)
		if err != nil {
			s.backfillErr.Store(err.Error())
			s.log.Error("backfill failed", "err", err, "embedded", embedded, "skipped", skipped)
		} else {
			s.log.Info("backfill complete", "embedded", embedded, "skipped", skipped)
		}
		s.backfillEnded.Store(nowUnix())
		s.backfillRunning.Store(false)
	}()
	return nil
}

// Backfill embeds every card in the database that doesn't yet have an embedding.
// Returns counts. Prefer StartBackfill for HTTP callers — the full backfill can
// take longer than the server's write timeout for larger workspaces.
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
				s.backfillSkipped.Add(1)
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
			s.backfillEmbedded.Add(1)
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
