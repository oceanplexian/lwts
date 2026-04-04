package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	channelCap       = 1000
	maxConcurrent    = 20
	deliveryTimeout  = 5 * time.Second
	retryPollInterval = 15 * time.Second
	maxAttempts      = 3
	autoDisableLimit = 500
)

// Backoff durations per attempt (0-indexed): immediate, 30s, 5min.
var backoffs = []time.Duration{0, 30 * time.Second, 5 * time.Minute}

// Dispatcher receives events and delivers them to matching webhooks.
type Dispatcher struct {
	store     *Store
	events    chan Event
	sem       chan struct{} // concurrency semaphore
	client    *http.Client
	logger    *slog.Logger
	stopCh    chan struct{}
	wg        sync.WaitGroup

	// BoardNameFunc resolves board ID → name for payload envelope.
	BoardNameFunc func(ctx context.Context, boardID string) string
}

func NewDispatcher(store *Store, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{
		store:  store,
		events: make(chan Event, channelCap),
		sem:    make(chan struct{}, maxConcurrent),
		client: &http.Client{Timeout: deliveryTimeout},
		logger: logger,
		stopCh: make(chan struct{}),
		BoardNameFunc: func(ctx context.Context, boardID string) string { return boardID },
	}
}

// Emit queues an event for dispatch. Non-blocking; drops if channel full.
func (d *Dispatcher) Emit(boardID, eventType string, data any) {
	select {
	case d.events <- Event{BoardID: boardID, EventType: eventType, Data: data}:
	default:
		d.logger.Warn("webhook event channel full, dropping event",
			"board_id", boardID, "event_type", eventType)
	}
}

// Run starts the dispatcher loop and retry worker. Call Stop() to shut down.
func (d *Dispatcher) Run() {
	d.wg.Add(2)
	go d.dispatchLoop()
	go d.retryLoop()
}

// Stop gracefully shuts down the dispatcher.
func (d *Dispatcher) Stop() {
	close(d.stopCh)
	d.wg.Wait()
}

func (d *Dispatcher) dispatchLoop() {
	defer d.wg.Done()
	for {
		select {
		case <-d.stopCh:
			return
		case evt := <-d.events:
			d.handleEvent(evt)
		}
	}
}

func (d *Dispatcher) handleEvent(evt Event) {
	ctx := context.Background()
	webhooks, err := d.store.ListEnabledForBoard(ctx, evt.BoardID, evt.EventType)
	if err != nil {
		d.logger.Error("list webhooks", "error", err, "board_id", evt.BoardID)
		return
	}

	for _, wh := range webhooks {
		payload := d.buildPayload(ctx, evt, wh)
		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			d.logger.Error("marshal payload", "error", err)
			continue
		}

		delivery, err := d.store.CreateDelivery(ctx, wh.ID, evt.EventType, string(bodyBytes))
		if err != nil {
			d.logger.Error("create delivery", "error", err)
			continue
		}

		// Check auto-disable threshold
		pending, _ := d.store.CountPending(ctx, wh.ID)
		if pending >= autoDisableLimit {
			disabled := false
			_, _ = d.store.UpdateWebhook(ctx, wh.ID, WebhookUpdate{Enabled: &disabled})
			d.logger.Warn("auto-disabled webhook", "webhook_id", wh.ID, "pending", pending)
			continue
		}

		// Dispatch in goroutine with semaphore
		whCopy := wh
		delCopy := delivery
		bodyCopy := bodyBytes
		d.sem <- struct{}{} // acquire
		go func() {
			defer func() { <-d.sem }() // release
			d.deliver(ctx, whCopy, delCopy, bodyCopy)
		}()
	}
}

func (d *Dispatcher) buildPayload(ctx context.Context, evt Event, wh Webhook) Payload {
	p := Payload{
		ID:        uuid.New().String(),
		Event:     evt.EventType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      evt.Data,
	}
	p.Board.ID = evt.BoardID
	p.Board.Name = d.BoardNameFunc(ctx, evt.BoardID)
	return p
}

func (d *Dispatcher) deliver(ctx context.Context, wh Webhook, delivery Delivery, body []byte) {
	sig := Sign(wh.Secret, body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewReader(body))
	if err != nil {
		d.logger.Error("build request", "error", err, "url", wh.URL)
		_ = d.store.MarkFailed(ctx, delivery.ID, 0, err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-LWTS-Signature", sig)
	req.Header.Set("X-LWTS-Event", delivery.EventType)
	req.Header.Set("X-LWTS-Delivery", delivery.ID)

	resp, err := d.client.Do(req)
	if err != nil {
		d.scheduleRetryOrFail(ctx, delivery, 0, err.Error())
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_ = d.store.MarkDelivered(ctx, delivery.ID, resp.StatusCode, string(respBody))
	} else {
		d.scheduleRetryOrFail(ctx, delivery, resp.StatusCode, string(respBody))
	}
}

func (d *Dispatcher) scheduleRetryOrFail(ctx context.Context, delivery Delivery, code int, body string) {
	attempt := delivery.Attempts // 0-indexed current attempt
	if attempt+1 >= maxAttempts {
		_ = d.store.MarkFailed(ctx, delivery.ID, code, body)
		return
	}

	nextBackoff := backoffs[attempt+1]
	nextRetry := time.Now().UTC().Add(nextBackoff)
	_ = d.store.MarkRetry(ctx, delivery.ID, code, body, nextRetry)
}

func (d *Dispatcher) retryLoop() {
	defer d.wg.Done()
	ticker := time.NewTicker(retryPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.processRetries()
		}
	}
}

func (d *Dispatcher) processRetries() {
	ctx := context.Background()
	deliveries, err := d.store.PendingRetries(ctx)
	if err != nil {
		d.logger.Error("fetch pending retries", "error", err)
		return
	}

	for _, del := range deliveries {
		wh, err := d.store.GetWebhook(ctx, del.WebhookID)
		if err != nil {
			d.logger.Error("get webhook for retry", "error", err, "webhook_id", del.WebhookID)
			_ = d.store.MarkFailed(ctx, del.ID, 0, fmt.Sprintf("webhook lookup failed: %v", err))
			continue
		}
		if !wh.Enabled {
			_ = d.store.MarkFailed(ctx, del.ID, 0, "webhook disabled")
			continue
		}

		del := del
		bodyBytes := []byte(del.Payload)
		d.sem <- struct{}{}
		go func() {
			defer func() { <-d.sem }()
			d.deliver(ctx, wh, del, bodyBytes)
		}()
	}
}
