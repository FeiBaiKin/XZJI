-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, client, ttl_seconds, ip, user_agent, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetActiveSession :one
SELECT s.id, s.user_id, s.client, s.ttl_seconds, s.expires_at, u.role, u.status
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.id = ? AND s.expires_at > ?;

-- name: ExtendSession :exec
UPDATE sessions SET expires_at = ? WHERE id = ?;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = ?;

-- name: DeleteUserSessionsExcept :exec
DELETE FROM sessions WHERE user_id = ? AND id <> ?;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at <= ?;
