-- name: CreateMedia :execlastid
INSERT INTO media (user_id, path, mime, size, width, height, color)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetMedia :one
SELECT * FROM media WHERE id = ?;

-- name: ListMediaByIDs :many
SELECT * FROM media WHERE id IN (sqlc.slice(ids));
