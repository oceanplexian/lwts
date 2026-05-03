UPDATE boards
SET columns = CAST(
    REPLACE(
        REPLACE(
            CAST(columns AS TEXT),
            ',{"id":"wont-do","label":"Won''t Do","color":"#94a3b8","type":"done"}',
            ''
        ),
        ',{"id":"wont-do","label":"Won''t Do"}',
        ''
    ) AS JSONB
)
WHERE CAST(columns AS TEXT) LIKE '%"wont-do"%';
