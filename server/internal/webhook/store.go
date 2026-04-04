package webhook

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/google/uuid"
)

// Store handles webhook and delivery persistence.
type Store struct {
	ds db.Datasource
}

func NewStore(ds db.Datasource) *Store {
	return &Store{ds: ds}
}

func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Store) CreateWebhook(ctx context.Context, boardID, url, eventType string) (Webhook, error) {
	id := uuid.New().String()
	secret, err := generateSecret()
	if err != nil {
		return Webhook{}, err
	}
	now := time.Now().UTC()

	_, err = s.ds.Exec(ctx,
		`INSERT INTO webhooks (id, board_id, event_type, url, secret, enabled, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, boardID, eventType, url, secret, 1, now,
	)
	if err != nil {
		return Webhook{}, err
	}

	return Webhook{
		ID: id, BoardID: boardID, EventType: eventType,
		URL: url, Secret: secret, Enabled: true, CreatedAt: now,
	}, nil
}

func (s *Store) GetWebhook(ctx context.Context, id string) (Webhook, error) {
	row := s.ds.QueryRow(ctx,
		`SELECT id, board_id, event_type, url, secret, enabled, created_at
		 FROM webhooks WHERE id = $1`, id)
	var w Webhook
	var enabled int
	err := row.Scan(&w.ID, &w.BoardID, &w.EventType, &w.URL, &w.Secret, &enabled, &w.CreatedAt)
	if err == db.ErrNoRows {
		return Webhook{}, db.ErrNoRows
	}
	w.Enabled = enabled != 0
	return w, err
}

func (s *Store) ListWebhooks(ctx context.Context, boardID string) ([]Webhook, error) {
	rows, err := s.ds.Query(ctx,
		`SELECT id, board_id, event_type, url, secret, enabled, created_at
		 FROM webhooks WHERE board_id = $1 ORDER BY created_at`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []Webhook
	for rows.Next() {
		var w Webhook
		var enabled int
		if err := rows.Scan(&w.ID, &w.BoardID, &w.EventType, &w.URL, &w.Secret, &enabled, &w.CreatedAt); err != nil {
			return nil, err
		}
		w.Enabled = enabled != 0
		webhooks = append(webhooks, w)
	}
	return webhooks, rows.Err()
}

// ListEnabledForBoard returns enabled webhooks matching the given event type.
func (s *Store) ListEnabledForBoard(ctx context.Context, boardID, eventType string) ([]Webhook, error) {
	rows, err := s.ds.Query(ctx,
		`SELECT id, board_id, event_type, url, secret, enabled, created_at
		 FROM webhooks WHERE board_id = $1 AND enabled = $2 AND (event_type = $3 OR event_type = $4)`,
		boardID, 1, eventType, EventWildcard)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []Webhook
	for rows.Next() {
		var w Webhook
		var enabled int
		if err := rows.Scan(&w.ID, &w.BoardID, &w.EventType, &w.URL, &w.Secret, &enabled, &w.CreatedAt); err != nil {
			return nil, err
		}
		w.Enabled = enabled != 0
		webhooks = append(webhooks, w)
	}
	return webhooks, rows.Err()
}

type WebhookUpdate struct {
	URL       *string
	EventType *string
	Enabled   *bool
}

func (s *Store) UpdateWebhook(ctx context.Context, id string, fields WebhookUpdate) (Webhook, error) {
	var sets []string
	var args []any
	argN := 1

	if fields.URL != nil {
		sets = append(sets, "url = $"+strconv.Itoa(argN))
		args = append(args, *fields.URL)
		argN++
	}
	if fields.EventType != nil {
		sets = append(sets, "event_type = $"+strconv.Itoa(argN))
		args = append(args, *fields.EventType)
		argN++
	}
	if fields.Enabled != nil {
		sets = append(sets, "enabled = $"+strconv.Itoa(argN))
		args = append(args, *fields.Enabled)
		argN++
	}

	if len(sets) == 0 {
		return s.GetWebhook(ctx, id)
	}

	args = append(args, id)
	query := "UPDATE webhooks SET " + strings.Join(sets, ", ") + " WHERE id = $" + strconv.Itoa(argN)

	n, err := s.ds.Exec(ctx, query, args...)
	if err != nil {
		return Webhook{}, err
	}
	if n == 0 {
		return Webhook{}, db.ErrNoRows
	}
	return s.GetWebhook(ctx, id)
}

func (s *Store) DeleteWebhook(ctx context.Context, id string) error {
	n, err := s.ds.Exec(ctx, `DELETE FROM webhooks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return db.ErrNoRows
	}
	return nil
}

// ── Deliveries ──

func (s *Store) CreateDelivery(ctx context.Context, webhookID, eventType, payload string) (Delivery, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	_, err := s.ds.Exec(ctx,
		`INSERT INTO webhook_deliveries (id, webhook_id, event_type, payload, status, attempts, next_retry_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, webhookID, eventType, payload, "pending", 0, now,
	)
	if err != nil {
		return Delivery{}, err
	}

	return Delivery{
		ID: id, WebhookID: webhookID, EventType: eventType,
		Payload: payload, Status: "pending", Attempts: 0, NextRetryAt: &now,
	}, nil
}

func (s *Store) GetDelivery(ctx context.Context, id string) (Delivery, error) {
	row := s.ds.QueryRow(ctx,
		`SELECT id, webhook_id, event_type, payload, response_code, response_body,
		        delivered_at, attempts, next_retry_at, status
		 FROM webhook_deliveries WHERE id = $1`, id)

	var d Delivery
	var deliveredAt, nextRetryAt *string
	err := row.Scan(&d.ID, &d.WebhookID, &d.EventType, &d.Payload,
		&d.ResponseCode, &d.ResponseBody, &deliveredAt,
		&d.Attempts, &nextRetryAt, &d.Status)
	if err == db.ErrNoRows {
		return Delivery{}, db.ErrNoRows
	}
	if err != nil {
		return Delivery{}, err
	}
	d.DeliveredAt = parseNullableTime(deliveredAt)
	d.NextRetryAt = parseNullableTime(nextRetryAt)
	return d, nil
}

func (s *Store) ListDeliveries(ctx context.Context, webhookID string, limit int) ([]Delivery, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.ds.Query(ctx,
		`SELECT id, webhook_id, event_type, payload, response_code, response_body,
		        delivered_at, attempts, next_retry_at, status
		 FROM webhook_deliveries WHERE webhook_id = $1
		 ORDER BY COALESCE(delivered_at, next_retry_at) DESC LIMIT $2`,
		webhookID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deliveries []Delivery
	for rows.Next() {
		var d Delivery
		var deliveredAt, nextRetryAt *string
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.EventType, &d.Payload,
			&d.ResponseCode, &d.ResponseBody, &deliveredAt,
			&d.Attempts, &nextRetryAt, &d.Status); err != nil {
			return nil, err
		}
		d.DeliveredAt = parseNullableTime(deliveredAt)
		d.NextRetryAt = parseNullableTime(nextRetryAt)
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}

func (s *Store) MarkDelivered(ctx context.Context, id string, code int, body string) error {
	now := time.Now().UTC()
	_, err := s.ds.Exec(ctx,
		`UPDATE webhook_deliveries
		 SET status = $1, response_code = $2, response_body = $3, delivered_at = $4,
		     attempts = attempts + 1, next_retry_at = NULL
		 WHERE id = $5`,
		"delivered", code, truncate(body, 4096), now, id)
	return err
}

func (s *Store) MarkFailed(ctx context.Context, id string, code int, body string) error {
	_, err := s.ds.Exec(ctx,
		`UPDATE webhook_deliveries
		 SET status = $1, response_code = $2, response_body = $3,
		     attempts = attempts + 1, next_retry_at = NULL
		 WHERE id = $4`,
		"failed", code, truncate(body, 4096), id)
	return err
}

func (s *Store) MarkRetry(ctx context.Context, id string, code int, body string, nextRetry time.Time) error {
	_, err := s.ds.Exec(ctx,
		`UPDATE webhook_deliveries
		 SET response_code = $1, response_body = $2, attempts = attempts + 1, next_retry_at = $3
		 WHERE id = $4`,
		code, truncate(body, 4096), nextRetry, id)
	return err
}

// PendingRetries returns deliveries with status=pending and next_retry_at <= now.
func (s *Store) PendingRetries(ctx context.Context) ([]Delivery, error) {
	now := time.Now().UTC()
	rows, err := s.ds.Query(ctx,
		`SELECT id, webhook_id, event_type, payload, response_code, response_body,
		        delivered_at, attempts, next_retry_at, status
		 FROM webhook_deliveries
		 WHERE status = $1 AND next_retry_at <= $2
		 ORDER BY next_retry_at LIMIT 100`,
		"pending", now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deliveries []Delivery
	for rows.Next() {
		var d Delivery
		var deliveredAt, nextRetryAt *string
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.EventType, &d.Payload,
			&d.ResponseCode, &d.ResponseBody, &deliveredAt,
			&d.Attempts, &nextRetryAt, &d.Status); err != nil {
			return nil, err
		}
		d.DeliveredAt = parseNullableTime(deliveredAt)
		d.NextRetryAt = parseNullableTime(nextRetryAt)
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}

// CountPending returns the number of pending deliveries for a webhook.
func (s *Store) CountPending(ctx context.Context, webhookID string) (int, error) {
	row := s.ds.QueryRow(ctx,
		`SELECT COUNT(*) FROM webhook_deliveries WHERE webhook_id = $1 AND status = $2`,
		webhookID, "pending")
	var count int
	err := row.Scan(&count)
	return count, err
}

func parseNullableTime(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z"} {
		if t, err := time.Parse(layout, *s); err == nil {
			return &t
		}
	}
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
