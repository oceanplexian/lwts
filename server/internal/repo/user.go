package repo

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/google/uuid"
)

var avatarColors = []string{
	"#82B1FF", "#fbc02d", "#4ade80", "#fb8c00",
	"#f44336", "#ce93d8", "#4dd0e1", "#ff8a65",
}

type UserRepository struct {
	ds db.Datasource
}

func NewUserRepository(ds db.Datasource) *UserRepository {
	return &UserRepository{ds: ds}
}

func deriveInitials(name string) string {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return strings.ToUpper(string([]rune(parts[0])[0:1]))
	}
	first := string([]rune(parts[0])[0:1])
	last := string([]rune(parts[len(parts)-1])[0:1])
	return strings.ToUpper(first + last)
}

func (r *UserRepository) Create(ctx context.Context, name, email, passwordHash string) (User, error) {
	id := uuid.New().String()
	initials := deriveInitials(name)
	now := time.Now().UTC()
	emailLower := strings.ToLower(strings.TrimSpace(email))
	nameTrimmed := strings.TrimSpace(name)

	// Pick color based on hash of email to be deterministic
	colorIdx := 0
	for _, c := range emailLower {
		colorIdx += int(c)
	}
	avatarColor := avatarColors[colorIdx%len(avatarColors)]

	_, err := r.ds.Exec(ctx,
		`INSERT INTO users (id, email, name, password_hash, avatar_color, avatar_url, initials, role, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		id, emailLower, nameTrimmed, passwordHash, avatarColor, "", initials, "member", now, now,
	)
	if err != nil {
		return User{}, err
	}

	return User{
		ID:           id,
		Email:        emailLower,
		Name:         nameTrimmed,
		PasswordHash: passwordHash,
		AvatarColor:  avatarColor,
		AvatarURL:    "",
		Initials:     initials,
		Role:         "member",
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (User, error) {
	row := r.ds.QueryRow(ctx,
		`SELECT id, email, name, password_hash, avatar_color, avatar_url, initials, role, welcomed, created_at, updated_at
		 FROM users WHERE id = $1`, id)

	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.AvatarColor, &u.AvatarURL, &u.Initials, &u.Role, &u.Welcomed, &u.CreatedAt, &u.UpdatedAt)
	if err == db.ErrNoRows {
		return User{}, ErrNotFound
	}
	return u, err
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (User, error) {
	row := r.ds.QueryRow(ctx,
		`SELECT id, email, name, password_hash, avatar_color, avatar_url, initials, role, welcomed, created_at, updated_at
		 FROM users WHERE email = $1`, strings.ToLower(strings.TrimSpace(email)))

	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.AvatarColor, &u.AvatarURL, &u.Initials, &u.Role, &u.Welcomed, &u.CreatedAt, &u.UpdatedAt)
	if err == db.ErrNoRows {
		return User{}, ErrNotFound
	}
	return u, err
}

type UserUpdate struct {
	Name        *string
	Email       *string
	AvatarColor *string
	AvatarURL   *string
	Role        *string
	Welcomed    *bool
}

func (r *UserRepository) Update(ctx context.Context, id string, fields UserUpdate) (User, error) {
	var sets []string
	var args []any
	argN := 1

	if fields.Name != nil {
		sets = append(sets, "name = $"+itoa(argN))
		args = append(args, strings.TrimSpace(*fields.Name))
		argN++
	}
	if fields.Email != nil {
		sets = append(sets, "email = $"+itoa(argN))
		args = append(args, strings.ToLower(strings.TrimSpace(*fields.Email)))
		argN++
	}
	if fields.AvatarColor != nil {
		sets = append(sets, "avatar_color = $"+itoa(argN))
		args = append(args, *fields.AvatarColor)
		argN++
	}
	if fields.AvatarURL != nil {
		sets = append(sets, "avatar_url = $"+itoa(argN))
		args = append(args, *fields.AvatarURL)
		argN++
	}
	if fields.Role != nil {
		sets = append(sets, "role = $"+itoa(argN))
		args = append(args, *fields.Role)
		argN++
	}
	if fields.Welcomed != nil {
		sets = append(sets, "welcomed = $"+itoa(argN))
		args = append(args, *fields.Welcomed)
		argN++
	}

	if len(sets) == 0 {
		return r.GetByID(ctx, id)
	}

	// Update initials if name changed
	if fields.Name != nil {
		initials := deriveInitials(*fields.Name)
		sets = append(sets, "initials = $"+itoa(argN))
		args = append(args, initials)
		argN++
	}

	now := time.Now().UTC()
	sets = append(sets, "updated_at = $"+itoa(argN))
	args = append(args, now)
	argN++

	args = append(args, id)
	query := "UPDATE users SET " + strings.Join(sets, ", ") + " WHERE id = $" + itoa(argN)

	n, err := r.ds.Exec(ctx, query, args...)
	if err != nil {
		return User{}, err
	}
	if n == 0 {
		return User{}, ErrNotFound
	}

	return r.GetByID(ctx, id)
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	// Nullify FK references before deleting
	_, _ = r.ds.Exec(ctx, `UPDATE cards SET assignee_id = NULL WHERE assignee_id = $1`, id)
	_, _ = r.ds.Exec(ctx, `UPDATE cards SET reporter_id = NULL WHERE reporter_id = $1`, id)
	_, _ = r.ds.Exec(ctx, `UPDATE boards SET owner_id = (SELECT id FROM users WHERE role = 'owner' LIMIT 1) WHERE owner_id = $1`, id)
	_, _ = r.ds.Exec(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1`, id)
	_, _ = r.ds.Exec(ctx, `DELETE FROM settings WHERE user_id = $1`, id)
	_, _ = r.ds.Exec(ctx, `DELETE FROM api_keys WHERE user_id = $1`, id)
	_, _ = r.ds.Exec(ctx, `DELETE FROM comments WHERE author_id = $1`, id)
	_, err := r.ds.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

func (r *UserRepository) List(ctx context.Context) ([]User, error) {
	rows, err := r.ds.Query(ctx,
		`SELECT id, email, name, avatar_color, avatar_url, initials, role, welcomed, created_at, updated_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.AvatarColor, &u.AvatarURL, &u.Initials, &u.Role, &u.Welcomed, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.ds.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
