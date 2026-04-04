package repo

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/google/uuid"
)

type CardRepository struct {
	ds db.Datasource
}

func NewCardRepository(ds db.Datasource) *CardRepository {
	return &CardRepository{ds: ds}
}

func (r *CardRepository) nextKey(ctx context.Context, boardID string) (string, error) {
	// Get the board's project_key
	row := r.ds.QueryRow(ctx, `SELECT project_key FROM boards WHERE id = $1`, boardID)
	var projectKey string
	if err := row.Scan(&projectKey); err != nil {
		if err == db.ErrNoRows {
			return "", fmt.Errorf("board not found: %s", boardID)
		}
		return "", err
	}

	// Get the max key number for this board
	row = r.ds.QueryRow(ctx, `SELECT COUNT(*) FROM cards WHERE board_id = $1`, boardID)
	var count int
	if err := row.Scan(&count); err != nil {
		return "", err
	}

	return projectKey + "-" + strconv.Itoa(count+1), nil
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
	id := uuid.New().String()
	now := time.Now().UTC()

	key, err := r.nextKey(ctx, boardID)
	if err != nil {
		return Card{}, fmt.Errorf("generate key: %w", err)
	}

	// Get max position in the target column
	row := r.ds.QueryRow(ctx,
		`SELECT COALESCE(MAX(position), -1) FROM cards WHERE board_id = $1 AND column_id = $2`,
		boardID, c.ColumnID)
	var maxPos int
	if err := row.Scan(&maxPos); err != nil {
		return Card{}, err
	}
	position := maxPos + 1

	tag := c.Tag
	if tag == "" {
		tag = "blue"
	}
	priority := c.Priority
	if priority == "" {
		priority = "medium"
	}

	_, err = r.ds.Exec(ctx,
		`INSERT INTO cards (id, board_id, column_id, title, description, tag, priority, assignee_id, reporter_id, points, position, key, version, due_date, related_card_ids, blocked_card_ids, epic_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)`,
		id, boardID, c.ColumnID, c.Title, c.Description, tag, priority,
		c.AssigneeID, c.ReporterID, c.Points, position, key, 1, c.DueDate, "[]", "[]", c.EpicID, now, now,
	)
	if err != nil {
		return Card{}, err
	}

	return Card{
		ID:             id,
		BoardID:        boardID,
		ColumnID:       c.ColumnID,
		Title:          c.Title,
		Description:    c.Description,
		Tag:            tag,
		Priority:       priority,
		AssigneeID:     c.AssigneeID,
		ReporterID:     c.ReporterID,
		Points:         c.Points,
		Position:       position,
		Key:            key,
		Version:        1,
		DueDate:        c.DueDate,
		RelatedCardIDs: "[]",
		BlockedCardIDs: "[]",
		EpicID:         c.EpicID,
		CreatedAt:      now,
		UpdatedAt:      now,
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
	// Get current card
	card, err := r.GetByID(ctx, id)
	if err != nil {
		return Card{}, err
	}
	if card.Version != version {
		return Card{}, ErrConflict
	}

	tx, err := r.ds.Begin(ctx)
	if err != nil {
		return Card{}, err
	}
	defer tx.Rollback(ctx)

	// Shift cards in the target column to make room
	_, err = tx.Exec(ctx,
		`UPDATE cards SET position = position + 1
		 WHERE board_id = $1 AND column_id = $2 AND position >= $3 AND id != $4`,
		card.BoardID, toColumn, position, id)
	if err != nil {
		return Card{}, err
	}

	// Move the card
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
	n, err := r.ds.Exec(ctx, `DELETE FROM cards WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
