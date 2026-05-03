-- Backfill the wont-do column onto existing boards that don't have it yet.
-- Both Postgres and SQLite store the columns array as a JSON document
-- (JSONB / TEXT respectively), so we rewrite it via a portable text REPLACE
-- on the trailing `]` of the array.
UPDATE boards
SET columns = CAST(
    REPLACE(
        CAST(columns AS TEXT),
        ']',
        ',{"id":"wont-do","label":"Won''t Do","color":"#94a3b8","type":"done"}]'
    ) AS JSONB
)
WHERE CAST(columns AS TEXT) NOT LIKE '%"wont-do"%';
