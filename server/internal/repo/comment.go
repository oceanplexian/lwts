package repo

import (
	"context"
	"time"

	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/google/uuid"
)

type CommentRepository struct {
	ds db.Datasource
}

func NewCommentRepository(ds db.Datasource) *CommentRepository {
	return &CommentRepository{ds: ds}
}

func (r *CommentRepository) Create(ctx context.Context, cardID, authorID, body string) (Comment, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	_, err := r.ds.Exec(ctx,
		`INSERT INTO comments (id, card_id, author_id, body, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		id, cardID, authorID, body, now, now,
	)
	if err != nil {
		return Comment{}, err
	}

	return Comment{
		ID:        id,
		CardID:    cardID,
		AuthorID:  authorID,
		Body:      body,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (r *CommentRepository) ListByCard(ctx context.Context, cardID string) ([]Comment, error) {
	rows, err := r.ds.Query(ctx,
		`SELECT id, card_id, author_id, body, created_at, updated_at
		 FROM comments WHERE card_id = $1 ORDER BY created_at ASC`, cardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.CardID, &c.AuthorID, &c.Body, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

func (r *CommentRepository) GetByID(ctx context.Context, id string) (Comment, error) {
	row := r.ds.QueryRow(ctx,
		`SELECT id, card_id, author_id, body, created_at, updated_at
		 FROM comments WHERE id = $1`, id)

	var c Comment
	err := row.Scan(&c.ID, &c.CardID, &c.AuthorID, &c.Body, &c.CreatedAt, &c.UpdatedAt)
	if err == db.ErrNoRows {
		return Comment{}, ErrNotFound
	}
	return c, err
}

func (r *CommentRepository) Update(ctx context.Context, id, body string) (Comment, error) {
	now := time.Now().UTC()
	n, err := r.ds.Exec(ctx,
		`UPDATE comments SET body = $1, updated_at = $2 WHERE id = $3`,
		body, now, id)
	if err != nil {
		return Comment{}, err
	}
	if n == 0 {
		return Comment{}, ErrNotFound
	}
	return r.GetByID(ctx, id)
}

func (r *CommentRepository) Delete(ctx context.Context, id string) error {
	n, err := r.ds.Exec(ctx, `DELETE FROM comments WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
