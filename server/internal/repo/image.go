package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/oceanplexian/lwts/server/internal/db"
)

type CardImageRepository struct {
	ds db.Datasource
}

func NewCardImageRepository(ds db.Datasource) *CardImageRepository {
	return &CardImageRepository{ds: ds}
}

// cardImageMetaColumns is the metadata projection — deliberately excludes the
// `data` BLOB so list/detail queries never haul image bytes into memory.
const cardImageMetaColumns = `id, card_id, filename, content_type, size_bytes, uploaded_by, created_at`

func scanCardImageMeta(s cardScanner) (CardImage, error) {
	var img CardImage
	if err := s.Scan(&img.ID, &img.CardID, &img.Filename, &img.ContentType,
		&img.SizeBytes, &img.UploadedBy, &img.CreatedAt); err != nil {
		return CardImage{}, err
	}
	return img, nil
}

func (r *CardImageRepository) Create(ctx context.Context, cardID, filename, contentType string, data []byte, uploadedBy *string) (CardImage, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	_, err := r.ds.Exec(ctx,
		`INSERT INTO card_images (id, card_id, filename, content_type, size_bytes, data, uploaded_by, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id, cardID, filename, contentType, len(data), data, uploadedBy, now,
	)
	if err != nil {
		return CardImage{}, err
	}

	return CardImage{
		ID:          id,
		CardID:      cardID,
		Filename:    filename,
		ContentType: contentType,
		SizeBytes:   len(data),
		UploadedBy:  uploadedBy,
		CreatedAt:   now,
	}, nil
}

// ListByCard returns image metadata for a card, oldest first. Bytes excluded.
func (r *CardImageRepository) ListByCard(ctx context.Context, cardID string) ([]CardImage, error) {
	rows, err := r.ds.Query(ctx,
		`SELECT `+cardImageMetaColumns+` FROM card_images WHERE card_id = $1 ORDER BY created_at ASC, id ASC`,
		cardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var imgs []CardImage
	for rows.Next() {
		img, err := scanCardImageMeta(rows)
		if err != nil {
			return nil, err
		}
		imgs = append(imgs, img)
	}
	return imgs, rows.Err()
}

// GetMeta returns a single image's metadata (no bytes).
func (r *CardImageRepository) GetMeta(ctx context.Context, id string) (CardImage, error) {
	row := r.ds.QueryRow(ctx, `SELECT `+cardImageMetaColumns+` FROM card_images WHERE id = $1`, id)
	img, err := scanCardImageMeta(row)
	if err == db.ErrNoRows {
		return CardImage{}, ErrNotFound
	}
	return img, err
}

// GetData returns an image's metadata plus its raw bytes for serving.
func (r *CardImageRepository) GetData(ctx context.Context, id string) (CardImage, []byte, error) {
	row := r.ds.QueryRow(ctx, `SELECT `+cardImageMetaColumns+`, data FROM card_images WHERE id = $1`, id)
	var img CardImage
	var data []byte
	err := row.Scan(&img.ID, &img.CardID, &img.Filename, &img.ContentType,
		&img.SizeBytes, &img.UploadedBy, &img.CreatedAt, &data)
	if err == db.ErrNoRows {
		return CardImage{}, nil, ErrNotFound
	}
	if err != nil {
		return CardImage{}, nil, err
	}
	return img, data, nil
}

func (r *CardImageRepository) Delete(ctx context.Context, id string) error {
	n, err := r.ds.Exec(ctx, `DELETE FROM card_images WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
