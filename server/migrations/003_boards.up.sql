CREATE TABLE boards (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    project_key TEXT NOT NULL DEFAULT 'LWTS',
    owner_id    UUID NOT NULL REFERENCES users(id),
    columns     JSONB NOT NULL DEFAULT '[{"id":"backlog","label":"Backlog"},{"id":"todo","label":"To Do"},{"id":"in-progress","label":"In Progress"},{"id":"done","label":"Done"}]',
    settings    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
