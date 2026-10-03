-- name: CreateUser :execlastid
INSERT INTO users (email, password_hash, nickname, role, status, inviter_id, email_verified_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = ?;

-- name: UpdateUserLastLogin :exec
UPDATE users SET last_login_at = ? WHERE id = ?;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = ? WHERE id = ?;

-- name: UpdateUserNickname :exec
UPDATE users SET nickname = ? WHERE id = ?;

-- name: UpdateUserRoleStatus :exec
UPDATE users SET role = ?, status = ? WHERE id = ?;

-- name: ListUsers :many
SELECT * FROM users
WHERE (sqlc.narg(keyword) IS NULL OR email LIKE sqlc.narg(keyword) OR nickname LIKE sqlc.narg(keyword))
  AND (sqlc.narg(role) IS NULL OR role = sqlc.narg(role))
  AND (sqlc.narg(status) IS NULL OR status = sqlc.narg(status))
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: CountUsers :one
SELECT COUNT(*) FROM users
WHERE (sqlc.narg(keyword) IS NULL OR email LIKE sqlc.narg(keyword) OR nickname LIKE sqlc.narg(keyword))
  AND (sqlc.narg(role) IS NULL OR role = sqlc.narg(role))
  AND (sqlc.narg(status) IS NULL OR status = sqlc.narg(status));

-- name: ListUserNicknames :many
SELECT id, nickname FROM users WHERE id IN (sqlc.slice(ids));
