package repo

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"-"` // never serialize
	AvatarColor  string    `json:"avatar_color"`
	AvatarURL    string    `json:"avatar_url"`
	Initials     string    `json:"initials"`
	Role         string    `json:"role"`
	Welcomed     bool      `json:"welcomed"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Board struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	ProjectKey string    `json:"project_key"`
	OwnerID    string    `json:"owner_id"`
	Columns    string    `json:"columns"` // JSON string
	Settings   string    `json:"settings"` // JSON string
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Card struct {
	ID             string    `json:"id"`
	BoardID        string    `json:"board_id"`
	ColumnID       string    `json:"column_id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Tag            string    `json:"tag"`
	Priority       string    `json:"priority"`
	AssigneeID     *string   `json:"assignee_id"`
	ReporterID     *string   `json:"reporter_id"`
	Points         *int      `json:"points"`
	Position       int       `json:"position"`
	Key            string    `json:"key"`
	Version        int       `json:"version"`
	DueDate        *string   `json:"due_date"`
	RelatedCardIDs string    `json:"related_card_ids"`
	BlockedCardIDs string    `json:"blocked_card_ids"`
	EpicID         *string   `json:"epic_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Comment struct {
	ID        string    `json:"id"`
	CardID    string    `json:"card_id"`
	AuthorID  string    `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
