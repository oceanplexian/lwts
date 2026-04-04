package webhook

import "time"

// Event type constants.
const (
	EventCardCreated   = "card.created"
	EventCardUpdated   = "card.updated"
	EventCardMoved     = "card.moved"
	EventCardCompleted = "card.completed"
	EventCardDeleted   = "card.deleted"
	EventCommentAdded  = "comment.added"
	EventWebhookTest   = "webhook.test"
	EventWildcard      = "*"
)

// AllEventTypes lists all concrete event types (excluding wildcard).
var AllEventTypes = []string{
	EventCardCreated, EventCardUpdated, EventCardMoved,
	EventCardCompleted, EventCardDeleted, EventCommentAdded,
}

type Webhook struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"board_id"`
	EventType string    `json:"event_type"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret,omitempty"` // omitted when masked
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

type Delivery struct {
	ID           string     `json:"id"`
	WebhookID    string     `json:"webhook_id"`
	EventType    string     `json:"event_type"`
	Payload      string     `json:"payload"`
	ResponseCode *int       `json:"response_code"`
	ResponseBody *string    `json:"response_body"`
	DeliveredAt  *time.Time `json:"delivered_at"`
	Attempts     int        `json:"attempts"`
	NextRetryAt  *time.Time `json:"next_retry_at"`
	Status       string     `json:"status"` // pending, delivered, failed
}

// Event is emitted by handlers and dispatched to matching webhooks.
type Event struct {
	BoardID   string `json:"board_id"`
	EventType string `json:"event_type"`
	Data      any    `json:"data"`
}

// Payload is the envelope sent to webhook URLs.
type Payload struct {
	ID        string `json:"id"`
	Event     string `json:"event"`
	Timestamp string `json:"timestamp"`
	Board     struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"board"`
	Data any `json:"data"`
}
