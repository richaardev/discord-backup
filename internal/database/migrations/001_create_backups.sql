-- +goose Up
CREATE TABLE IF NOT EXISTS backups (
    id         TEXT PRIMARY KEY,
    guild_id   TEXT NOT NULL,
    data       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- +goose Down
DROP TABLE IF EXISTS backups;
