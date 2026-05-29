-- +goose Up
ALTER TABLE backups ADD COLUMN created_by TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE backups DROP COLUMN created_by;
