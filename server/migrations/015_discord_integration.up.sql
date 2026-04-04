CREATE TABLE IF NOT EXISTS discord_integrations (
    id TEXT PRIMARY KEY,
    bot_token TEXT NOT NULL DEFAULT '',
    guild_id TEXT NOT NULL DEFAULT '',
    channel_id TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 0,
    notify TEXT NOT NULL DEFAULT '{"assigned":true,"done":true,"comment":true,"created":false,"priority":true}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
