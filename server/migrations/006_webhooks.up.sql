CREATE TABLE webhooks (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    board_id   UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    url        TEXT NOT NULL,
    secret     TEXT NOT NULL,
    enabled    BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_webhooks_board ON webhooks (board_id);
CREATE INDEX idx_webhooks_enabled ON webhooks (board_id, enabled) WHERE enabled = true;

CREATE TABLE webhook_deliveries (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id    UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_type    TEXT NOT NULL,
    payload       JSONB NOT NULL,
    response_code INT,
    response_body TEXT,
    delivered_at  TIMESTAMPTZ,
    attempts      INT NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ,
    status        TEXT NOT NULL DEFAULT 'pending'
);
CREATE INDEX idx_deliveries_webhook ON webhook_deliveries (webhook_id);
CREATE INDEX idx_deliveries_pending ON webhook_deliveries (status, next_retry_at) WHERE status = 'pending';
