-- name: CreateBackup :execresult
INSERT INTO backups (id, guild_id, data, created_by) VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET data = excluded.data, updated_at = datetime('now');

-- name: GetBackup :one
SELECT * FROM backups WHERE id = ?;

-- name: ListBackups :many
SELECT * FROM backups WHERE created_by = ? ORDER BY created_at DESC;

-- name: DeleteBackup :exec
DELETE FROM backups WHERE id = ?;
