CREATE TABLE board_events (
    id          BIGSERIAL PRIMARY KEY,
    board_id    UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    event_type  TEXT NOT NULL,
    payload     JSONB NOT NULL,
    sender_id   UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_board_events_board_id ON board_events (board_id, id);
CREATE INDEX idx_board_events_created_at ON board_events (created_at);
