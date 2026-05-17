package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/oceanplexian/lwts/server/internal/db"
)

// StoredEvent is a single persisted board event.
type StoredEvent struct {
	ID        int64     `json:"id"`
	BoardID   string    `json:"board_id"`
	EventType string    `json:"type"`
	Payload   []byte    `json:"-"`
	SenderID  string    `json:"sender_id,omitempty"`
	CreatedAt time.Time `json:"occurred_at"`
}

// EventStore persists board events for replay via Last-Event-ID.
// A nil EventStore means persistence is disabled and events are in-memory only.
type EventStore interface {
	Persist(ctx context.Context, boardID, eventType string, payload []byte, senderID string) (StoredEvent, error)
	LoadSince(ctx context.Context, boardID string, sinceID int64, limit int) ([]StoredEvent, error)
	MaxID(ctx context.Context, boardID string) (int64, error)
	PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// DBEventStore persists events to the board_events table.
type DBEventStore struct {
	ds db.Datasource
}

// NewDBEventStore returns an EventStore backed by the given datasource.
func NewDBEventStore(ds db.Datasource) *DBEventStore {
	return &DBEventStore{ds: ds}
}

func (s *DBEventStore) Persist(ctx context.Context, boardID, eventType string, payload []byte, senderID string) (StoredEvent, error) {
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	if !json.Valid(payload) {
		return StoredEvent{}, fmt.Errorf("payload is not valid JSON")
	}

	var (
		id        int64
		createdAt time.Time
		sender    any
	)
	if senderID != "" {
		sender = senderID
	}

	const insertSQL = `INSERT INTO board_events (board_id, event_type, payload, sender_id)
VALUES ($1, $2, $3, $4) RETURNING id, created_at`

	row := s.ds.QueryRow(ctx, insertSQL, boardID, eventType, string(payload), sender)
	if err := row.Scan(&id, &createdAt); err != nil {
		return StoredEvent{}, fmt.Errorf("persist event: %w", err)
	}

	return StoredEvent{
		ID:        id,
		BoardID:   boardID,
		EventType: eventType,
		Payload:   payload,
		SenderID:  senderID,
		CreatedAt: createdAt,
	}, nil
}

func (s *DBEventStore) LoadSince(ctx context.Context, boardID string, sinceID int64, limit int) ([]StoredEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	const querySQL = `SELECT id, event_type, payload, sender_id, created_at
FROM board_events
WHERE board_id = $1 AND id > $2
ORDER BY id ASC
LIMIT $3`

	rows, err := s.ds.Query(ctx, querySQL, boardID, sinceID, limit)
	if err != nil {
		return nil, fmt.Errorf("load events: %w", err)
	}
	defer rows.Close()

	var out []StoredEvent
	for rows.Next() {
		var (
			ev      StoredEvent
			payload string
			sender  *string
		)
		if err := rows.Scan(&ev.ID, &ev.EventType, &payload, &sender, &ev.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		ev.BoardID = boardID
		ev.Payload = []byte(payload)
		if sender != nil {
			ev.SenderID = *sender
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (s *DBEventStore) MaxID(ctx context.Context, boardID string) (int64, error) {
	const sqlText = `SELECT COALESCE(MAX(id), 0) FROM board_events WHERE board_id = $1`
	var max int64
	if err := s.ds.QueryRow(ctx, sqlText, boardID).Scan(&max); err != nil {
		return 0, fmt.Errorf("max event id: %w", err)
	}
	return max, nil
}

func (s *DBEventStore) PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	const sqlText = `DELETE FROM board_events WHERE created_at < $1`
	n, err := s.ds.Exec(ctx, sqlText, cutoff.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, fmt.Errorf("purge events: %w", err)
	}
	return n, nil
}
