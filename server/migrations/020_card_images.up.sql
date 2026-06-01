CREATE TABLE card_images (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id      UUID NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    filename     TEXT NOT NULL DEFAULT '',
    content_type TEXT NOT NULL,
    size_bytes   INT NOT NULL DEFAULT 0,
    data         BYTEA NOT NULL,
    uploaded_by  UUID REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_card_images_card ON card_images (card_id, created_at);
