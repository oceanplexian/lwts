CREATE TABLE cards (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    board_id    UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    column_id   TEXT NOT NULL,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    tag         TEXT NOT NULL DEFAULT 'blue',
    priority    TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('highest','high','medium','low','lowest')),
    assignee_id UUID REFERENCES users(id),
    reporter_id UUID REFERENCES users(id),
    points      INT,
    position    INT NOT NULL DEFAULT 0,
    key         TEXT NOT NULL,
    version     INT NOT NULL DEFAULT 1,
    due_date    DATE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_cards_board ON cards (board_id);
CREATE INDEX idx_cards_board_column ON cards (board_id, column_id);
CREATE INDEX idx_cards_assignee ON cards (assignee_id);
CREATE UNIQUE INDEX idx_cards_key ON cards (key);
