package repo

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/google/uuid"
)

// retryBackoff sleeps a small random duration to spread concurrent retries.
func retryBackoff(attempt int) {
	// 1-5ms * attempt, randomized
	ms := (1 + rand.IntN(5)) * (attempt + 1)
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

// querier is satisfied by both db.Datasource and db.Tx.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) *db.Row
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
	Query(ctx context.Context, sql string, args ...any) (*db.Rows, error)
}

type CardRepository struct {
	ds db.Datasource
}

func NewCardRepository(ds db.Datasource) *CardRepository {
	return &CardRepository{ds: ds}
}

const maxRetries = 5

// isRetryable returns true for Postgres errors that resolve on retry:
//   - 40P01: deadlock_detected (concurrent position shifts)
//   - 40001: serialization_failure
//   - 23505: unique_violation (key collision between nextKey and INSERT)
func isRetryable(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "40P01", "40001", "23505":
			return true
		}
	}
	return false
}

// nextKey generates the next card key in a dedicated short-lived transaction.
// It locks the board row with FOR UPDATE to serialize key generation, but
// commits immediately so the lock is held for only microseconds — this avoids
// connection pool starvation under high concurrency.
func (r *CardRepository) nextKey(ctx context.Context, boardID string) (string, error) {
	tx, err := r.ds.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	forUpdate := ""
	if r.ds.DBType() == "postgres" {
		forUpdate = " FOR UPDATE"
	}
	row := tx.QueryRow(ctx, `SELECT project_key FROM boards WHERE id = $1`+forUpdate, boardID)
	var projectKey string
	if err := row.Scan(&projectKey); err != nil {
		if err == db.ErrNoRows {
			return "", fmt.Errorf("board not found: %s", boardID)
		}
		return "", err
	}

	var maxKeySQL string
	if r.ds.DBType() == "postgres" {
		maxKeySQL = `SELECT COALESCE(MAX(CAST(SUBSTRING(key FROM '[0-9]+$') AS INTEGER)), 0) FROM cards WHERE board_id = $1`
	} else {
		maxKeySQL = `SELECT COALESCE(MAX(CAST(SUBSTR(key, INSTR(key, '-') + 1) AS INTEGER)), 0) FROM cards WHERE board_id = $1`
	}
	row = tx.QueryRow(ctx, maxKeySQL, boardID)
	var maxNum int
	if err := row.Scan(&maxNum); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}

	return projectKey + "-" + strconv.Itoa(maxNum+1), nil
}

type CardCreate struct {
	ColumnID    string
	Title       string
	Description string
	Tag         string
	Priority    string
	AssigneeID  *string
	ReporterID  *string
	Points      *int
	DueDate     *string
	EpicID      *string
}

func (r *CardRepository) Create(ctx context.Context, boardID string, c CardCreate) (Card, error) {
	tag := c.Tag
	if tag == "" {
		tag = "blue"
	}
	priority := c.Priority
	if priority == "" {
		priority = "medium"
	}
	for attempt := 0; attempt < maxRetries; attempt++ {
		card, err := r.createOnce(ctx, boardID, c, tag, priority)
		if err == nil {
			return card, nil
		}
		if isRetryable(err) && attempt < maxRetries-1 {
			retryBackoff(attempt)
			continue
		}
		return Card{}, err
	}
	return Card{}, fmt.Errorf("create failed after %d retries", maxRetries)
}

func (r *CardRepository) createOnce(ctx context.Context, boardID string, c CardCreate, tag, priority string) (Card, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	// Key generation uses its own short-lived transaction so the board row
	// lock is released before we begin the insert transaction.
	key, err := r.nextKey(ctx, boardID)
	if err != nil {
		return Card{}, fmt.Errorf("generate key: %w", err)
	}

	tx, err := r.ds.Begin(ctx)
	if err != nil {
		return Card{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(position), -1) FROM cards WHERE board_id = $1 AND column_id = $2`,
		boardID, c.ColumnID)
	var maxPos int
	if err := row.Scan(&maxPos); err != nil {
		return Card{}, err
	}
	position := maxPos + 1

	_, err = tx.Exec(ctx,
		`INSERT INTO cards (id, board_id, column_id, title, description, tag, priority, assignee_id, reporter_id, points, position, key, version, due_date, related_card_ids, blocked_card_ids, epic_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)`,
		id, boardID, c.ColumnID, c.Title, c.Description, tag, priority,
		c.AssigneeID, c.ReporterID, c.Points, position, key, 1, c.DueDate, "[]", "[]", c.EpicID, now, now,
	)
	if err != nil {
		return Card{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Card{}, err
	}

	return Card{
		ID: id, BoardID: boardID, ColumnID: c.ColumnID,
		Title: c.Title, Description: c.Description, Tag: tag, Priority: priority,
		AssigneeID: c.AssigneeID, ReporterID: c.ReporterID, Points: c.Points,
		Position: position, Key: key, Version: 1, DueDate: c.DueDate,
		RelatedCardIDs: "[]", BlockedCardIDs: "[]", EpicID: c.EpicID,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (r *CardRepository) GetByID(ctx context.Context, id string) (Card, error) {
	row := r.ds.QueryRow(ctx,
		`SELECT id, board_id, column_id, title, description, tag, priority, assignee_id, reporter_id,
		        points, position, key, version, due_date, related_card_ids, blocked_card_ids, epic_id, created_at, updated_at
		 FROM cards WHERE id = $1`, id)

	var c Card
	err := row.Scan(&c.ID, &c.BoardID, &c.ColumnID, &c.Title, &c.Description, &c.Tag, &c.Priority,
		&c.AssigneeID, &c.ReporterID, &c.Points, &c.Position, &c.Key, &c.Version, &c.DueDate,
		&c.RelatedCardIDs, &c.BlockedCardIDs, &c.EpicID, &c.CreatedAt, &c.UpdatedAt)
	if err == db.ErrNoRows {
		return Card{}, ErrNotFound
	}
	return c, err
}

func (r *CardRepository) ListByBoard(ctx context.Context, boardID string) ([]Card, error) {
	rows, err := r.ds.Query(ctx,
		`SELECT id, board_id, column_id, title, description, tag, priority, assignee_id, reporter_id,
		        points, position, key, version, due_date, related_card_ids, blocked_card_ids, epic_id, created_at, updated_at
		 FROM cards WHERE board_id = $1
		 ORDER BY column_id, position`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []Card
	for rows.Next() {
		var c Card
		if err := rows.Scan(&c.ID, &c.BoardID, &c.ColumnID, &c.Title, &c.Description, &c.Tag, &c.Priority,
			&c.AssigneeID, &c.ReporterID, &c.Points, &c.Position, &c.Key, &c.Version, &c.DueDate,
			&c.RelatedCardIDs, &c.BlockedCardIDs, &c.EpicID, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

type CardUpdate struct {
	Title          *string
	Description    *string
	Tag            *string
	Priority       *string
	AssigneeID     **string // double pointer: nil = don't update, non-nil = set (inner can be nil to clear)
	ReporterID     **string
	Points         **int
	DueDate        **string
	RelatedCardIDs *string
	BlockedCardIDs *string
	EpicID         **string
}

func (r *CardRepository) Update(ctx context.Context, id string, version int, fields CardUpdate) (Card, error) {
	var sets []string
	var args []any
	argN := 1

	if fields.Title != nil {
		sets = append(sets, "title = $"+strconv.Itoa(argN))
		args = append(args, *fields.Title)
		argN++
	}
	if fields.Description != nil {
		sets = append(sets, "description = $"+strconv.Itoa(argN))
		args = append(args, *fields.Description)
		argN++
	}
	if fields.Tag != nil {
		sets = append(sets, "tag = $"+strconv.Itoa(argN))
		args = append(args, *fields.Tag)
		argN++
	}
	if fields.Priority != nil {
		sets = append(sets, "priority = $"+strconv.Itoa(argN))
		args = append(args, *fields.Priority)
		argN++
	}
	if fields.AssigneeID != nil {
		sets = append(sets, "assignee_id = $"+strconv.Itoa(argN))
		args = append(args, *fields.AssigneeID)
		argN++
	}
	if fields.ReporterID != nil {
		sets = append(sets, "reporter_id = $"+strconv.Itoa(argN))
		args = append(args, *fields.ReporterID)
		argN++
	}
	if fields.Points != nil {
		sets = append(sets, "points = $"+strconv.Itoa(argN))
		args = append(args, *fields.Points)
		argN++
	}
	if fields.DueDate != nil {
		sets = append(sets, "due_date = $"+strconv.Itoa(argN))
		args = append(args, *fields.DueDate)
		argN++
	}
	if fields.RelatedCardIDs != nil {
		sets = append(sets, "related_card_ids = $"+strconv.Itoa(argN))
		args = append(args, *fields.RelatedCardIDs)
		argN++
	}
	if fields.BlockedCardIDs != nil {
		sets = append(sets, "blocked_card_ids = $"+strconv.Itoa(argN))
		args = append(args, *fields.BlockedCardIDs)
		argN++
	}
	if fields.EpicID != nil {
		sets = append(sets, "epic_id = $"+strconv.Itoa(argN))
		args = append(args, *fields.EpicID)
		argN++
	}

	if len(sets) == 0 {
		return r.GetByID(ctx, id)
	}

	now := time.Now().UTC()
	sets = append(sets, "updated_at = $"+strconv.Itoa(argN))
	args = append(args, now)
	argN++

	sets = append(sets, "version = version + 1")

	// Optimistic lock: WHERE id = $N AND version = $N+1
	args = append(args, id)
	idArg := strconv.Itoa(argN)
	argN++
	args = append(args, version)
	verArg := strconv.Itoa(argN)

	query := "UPDATE cards SET " + strings.Join(sets, ", ") + " WHERE id = $" + idArg + " AND version = $" + verArg

	n, err := r.ds.Exec(ctx, query, args...)
	if err != nil {
		return Card{}, err
	}
	if n == 0 {
		// Check if card exists at all
		_, err := r.GetByID(ctx, id)
		if err == ErrNotFound {
			return Card{}, ErrNotFound
		}
		return Card{}, ErrConflict
	}

	return r.GetByID(ctx, id)
}

type MoveOption struct {
	EpicID **string // nil = don't change, non-nil = set (inner can be nil to clear)
}

func (r *CardRepository) Move(ctx context.Context, id string, version int, toColumn string, position int, opts ...MoveOption) (Card, error) {
	for attempt := 0; attempt < maxRetries; attempt++ {
		card, err := r.moveOnce(ctx, id, version, toColumn, position, opts...)
		if err == nil {
			return card, nil
		}
		if err == ErrConflict || err == ErrNotFound {
			return Card{}, err
		}
		if isRetryable(err) && attempt < maxRetries-1 {
			retryBackoff(attempt)
			continue
		}
		return Card{}, err
	}
	return Card{}, fmt.Errorf("move failed after %d retries", maxRetries)
}

func (r *CardRepository) moveOnce(ctx context.Context, id string, version int, toColumn string, position int, opts ...MoveOption) (Card, error) {
	tx, err := r.ds.Begin(ctx)
	if err != nil {
		return Card{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Read card inside the transaction — no FOR UPDATE needed. We rely on the
	// version-checked UPDATE to detect conflicts and on automatic retry to
	// handle deadlocks from concurrent position shifts.
	row := tx.QueryRow(ctx,
		`SELECT board_id, version FROM cards WHERE id = $1`, id)
	var boardID string
	var curVersion int
	if err := row.Scan(&boardID, &curVersion); err != nil {
		if err == db.ErrNoRows {
			return Card{}, ErrNotFound
		}
		return Card{}, err
	}
	if curVersion != version {
		return Card{}, ErrConflict
	}

	// Shift cards in the target column to make room
	_, err = tx.Exec(ctx,
		`UPDATE cards SET position = position + 1
		 WHERE board_id = $1 AND column_id = $2 AND position >= $3 AND id != $4`,
		boardID, toColumn, position, id)
	if err != nil {
		return Card{}, err
	}

	// Move the card — version check ensures no lost updates
	now := time.Now().UTC()
	var n int64
	if len(opts) > 0 && opts[0].EpicID != nil {
		n, err = tx.Exec(ctx,
			`UPDATE cards SET column_id = $1, position = $2, epic_id = $3, version = version + 1, updated_at = $4
			 WHERE id = $5 AND version = $6`,
			toColumn, position, *opts[0].EpicID, now, id, version)
	} else {
		n, err = tx.Exec(ctx,
			`UPDATE cards SET column_id = $1, position = $2, version = version + 1, updated_at = $3
			 WHERE id = $4 AND version = $5`,
			toColumn, position, now, id, version)
	}
	if err != nil {
		return Card{}, err
	}
	if n == 0 {
		return Card{}, ErrConflict
	}

	if err := tx.Commit(ctx); err != nil {
		return Card{}, err
	}

	return r.GetByID(ctx, id)
}

func (r *CardRepository) Delete(ctx context.Context, id string) error {
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := r.deleteOnce(ctx, id)
		if err == nil || err == ErrNotFound {
			return err
		}
		if isRetryable(err) && attempt < maxRetries-1 {
			retryBackoff(attempt)
			continue
		}
		return err
	}
	return fmt.Errorf("delete failed after %d retries", maxRetries)
}

func (r *CardRepository) deleteOnce(ctx context.Context, id string) error {
	tx, err := r.ds.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `SELECT board_id, column_id, position FROM cards WHERE id = $1`, id)
	var boardID, colID string
	var pos int
	if err := row.Scan(&boardID, &colID, &pos); err != nil {
		if err == db.ErrNoRows {
			return ErrNotFound
		}
		return err
	}

	_, err = tx.Exec(ctx, `DELETE FROM cards WHERE id = $1`, id)
	if err != nil {
		return err
	}

	// Close the position gap
	_, err = tx.Exec(ctx,
		`UPDATE cards SET position = position - 1 WHERE board_id = $1 AND column_id = $2 AND position > $3`,
		boardID, colID, pos)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *CardRepository) BulkMove(ctx context.Context, ids []string, toColumn string) ([]Card, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	for attempt := 0; attempt < maxRetries; attempt++ {
		cards, err := r.bulkMoveOnce(ctx, ids, toColumn)
		if err == nil {
			return cards, nil
		}
		if isRetryable(err) && attempt < maxRetries-1 {
			retryBackoff(attempt)
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("bulk move failed after %d retries", maxRetries)
}

func (r *CardRepository) bulkMoveOnce(ctx context.Context, ids []string, toColumn string) ([]Card, error) {
	tx, err := r.ds.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `SELECT board_id FROM cards WHERE id = $1`, ids[0])
	var boardID string
	if err := row.Scan(&boardID); err != nil {
		return nil, err
	}

	row = tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(position), -1) FROM cards WHERE board_id = $1 AND column_id = $2`,
		boardID, toColumn)
	var maxPos int
	if err := row.Scan(&maxPos); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	for i, id := range ids {
		_, err = tx.Exec(ctx,
			`UPDATE cards SET column_id = $1, position = $2, version = version + 1, updated_at = $3 WHERE id = $4`,
			toColumn, maxPos+1+i, now, id)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	var result []Card
	for _, id := range ids {
		card, err := r.GetByID(ctx, id)
		if err != nil {
			continue
		}
		result = append(result, card)
	}
	return result, nil
}
