package auth

import (
	"context"
	"time"

	"github.com/oceanplexian/lwts/server/internal/repo"
)

// UserStore abstracts user persistence for the auth package.
type UserStore interface {
	CreateUser(ctx context.Context, email, name, passwordHash, avatarColor, initials, role string) (*repo.User, error)
	GetUserByEmail(ctx context.Context, email string) (*repo.User, error)
	GetUserByID(ctx context.Context, id string) (*repo.User, error)
	CountUsers(ctx context.Context) (int, error)
	UpdateUserRole(ctx context.Context, id, role string) error
	UpdateUser(ctx context.Context, id string, fields repo.UserUpdate) (*repo.User, error)
}

// TokenStore abstracts refresh token persistence.
type TokenStore interface {
	SaveRefreshToken(ctx context.Context, userID, jti string, expiresAt time.Time) error
	GetRefreshToken(ctx context.Context, jti string) (*RefreshTokenRecord, error)
	RevokeRefreshToken(ctx context.Context, jti string) error
	RevokeAllUserTokens(ctx context.Context, userID string) error
}

// RefreshTokenRecord represents a stored refresh token.
type RefreshTokenRecord struct {
	ID        string
	UserID    string
	JTI       string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// UserRepoAdapter adapts repo.UserRepository to the UserStore interface.
type UserRepoAdapter struct {
	repo *repo.UserRepository
}

func NewUserRepoAdapter(r *repo.UserRepository) *UserRepoAdapter {
	return &UserRepoAdapter{repo: r}
}

func (a *UserRepoAdapter) CreateUser(ctx context.Context, email, name, passwordHash, avatarColor, initials, role string) (*repo.User, error) {
	u, err := a.repo.Create(ctx, name, email, passwordHash)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (a *UserRepoAdapter) GetUserByEmail(ctx context.Context, email string) (*repo.User, error) {
	u, err := a.repo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (a *UserRepoAdapter) GetUserByID(ctx context.Context, id string) (*repo.User, error) {
	u, err := a.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (a *UserRepoAdapter) CountUsers(ctx context.Context) (int, error) {
	return a.repo.Count(ctx)
}

func (a *UserRepoAdapter) UpdateUserRole(ctx context.Context, id, role string) error {
	_, err := a.repo.Update(ctx, id, repo.UserUpdate{Role: &role})
	return err
}

func (a *UserRepoAdapter) UpdateUser(ctx context.Context, id string, fields repo.UserUpdate) (*repo.User, error) {
	u, err := a.repo.Update(ctx, id, fields)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
