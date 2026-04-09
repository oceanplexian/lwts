package repo

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/oceanplexian/lwts/server/internal/db"
)

type BoardRepository struct {
	ds db.Datasource
}

func NewBoardRepository(ds db.Datasource) *BoardRepository {
	return &BoardRepository{ds: ds}
}

func (r *BoardRepository) Create(ctx context.Context, name, projectKey, ownerID string) (Board, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	defaultCols := `[{"id":"backlog","label":"Backlog"},{"id":"todo","label":"To Do"},{"id":"in-progress","label":"In Progress"},{"id":"done","label":"Done"}]`
	defaultSettings := `{}`

	_, err := r.ds.Exec(ctx,
		`INSERT INTO boards (id, name, project_key, owner_id, columns, settings, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id, name, projectKey, ownerID, defaultCols, defaultSettings, now, now,
	)
	if err != nil {
		return Board{}, err
	}

	return Board{
		ID:         id,
		Name:       name,
		ProjectKey: projectKey,
		OwnerID:    ownerID,
		Columns:    defaultCols,
		Settings:   defaultSettings,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

func (r *BoardRepository) GetByID(ctx context.Context, id string) (Board, error) {
	row := r.ds.QueryRow(ctx,
		`SELECT id, name, project_key, owner_id, columns, settings, created_at, updated_at
		 FROM boards WHERE id = $1`, id)

	var b Board
	err := row.Scan(&b.ID, &b.Name, &b.ProjectKey, &b.OwnerID, &b.Columns, &b.Settings, &b.CreatedAt, &b.UpdatedAt)
	if err == db.ErrNoRows {
		return Board{}, ErrNotFound
	}
	return b, err
}

func (r *BoardRepository) List(ctx context.Context) ([]Board, error) {
	rows, err := r.ds.Query(ctx,
		`SELECT id, name, project_key, owner_id, columns, settings, created_at, updated_at FROM boards ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []Board
	for rows.Next() {
		var b Board
		if err := rows.Scan(&b.ID, &b.Name, &b.ProjectKey, &b.OwnerID, &b.Columns, &b.Settings, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	return boards, rows.Err()
}

type BoardUpdate struct {
	Name       *string
	ProjectKey *string
	Columns    *string
	Settings   *string
}

func (r *BoardRepository) Update(ctx context.Context, id string, fields BoardUpdate) (Board, error) {
	var sets []string
	var args []any
	argN := 1

	if fields.Name != nil {
		sets = append(sets, "name = $"+strconv.Itoa(argN))
		args = append(args, *fields.Name)
		argN++
	}
	if fields.ProjectKey != nil {
		sets = append(sets, "project_key = $"+strconv.Itoa(argN))
		args = append(args, *fields.ProjectKey)
		argN++
	}
	if fields.Columns != nil {
		sets = append(sets, "columns = $"+strconv.Itoa(argN))
		args = append(args, *fields.Columns)
		argN++
	}
	if fields.Settings != nil {
		sets = append(sets, "settings = $"+strconv.Itoa(argN))
		args = append(args, *fields.Settings)
		argN++
	}

	if len(sets) == 0 {
		return r.GetByID(ctx, id)
	}

	now := time.Now().UTC()
	sets = append(sets, "updated_at = $"+strconv.Itoa(argN))
	args = append(args, now)
	argN++

	args = append(args, id)
	query := "UPDATE boards SET " + strings.Join(sets, ", ") + " WHERE id = $" + strconv.Itoa(argN)

	n, err := r.ds.Exec(ctx, query, args...)
	if err != nil {
		return Board{}, err
	}
	if n == 0 {
		return Board{}, ErrNotFound
	}

	return r.GetByID(ctx, id)
}

func (r *BoardRepository) Delete(ctx context.Context, id string) error {
	n, err := r.ds.Exec(ctx, `DELETE FROM boards WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
