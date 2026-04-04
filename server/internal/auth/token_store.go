package auth

import (
	"context"
	"time"

	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/google/uuid"
)

// DBTokenStore implements TokenStore using a Datasource.
type DBTokenStore struct {
	ds db.Datasource
}

func NewDBTokenStore(ds db.Datasource) *DBTokenStore {
	return &DBTokenStore{ds: ds}
}

func (s *DBTokenStore) SaveRefreshToken(ctx context.Context, userID, jti string, expiresAt time.Time) error {
	id := uuid.New().String()
	_, err := s.ds.Exec(ctx,
		`INSERT INTO refresh_tokens (id, user_id, jti, expires_at, created_at) VALUES ($1, $2, $3, $4, $5)`,
		id, userID, jti, expiresAt, time.Now().UTC(),
	)
	return err
}

func (s *DBTokenStore) GetRefreshToken(ctx context.Context, jti string) (*RefreshTokenRecord, error) {
	row := s.ds.QueryRow(ctx,
		`SELECT id, user_id, jti, expires_at, created_at FROM refresh_tokens WHERE jti = $1`, jti)
	var r RefreshTokenRecord
	if err := row.Scan(&r.ID, &r.UserID, &r.JTI, &r.ExpiresAt, &r.CreatedAt); err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *DBTokenStore) RevokeRefreshToken(ctx context.Context, jti string) error {
	_, err := s.ds.Exec(ctx, `DELETE FROM refresh_tokens WHERE jti = $1`, jti)
	return err
}

func (s *DBTokenStore) RevokeAllUserTokens(ctx context.Context, userID string) error {
	_, err := s.ds.Exec(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1`, userID)
	return err
}
