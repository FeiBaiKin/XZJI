-- name: CreateMedia :execlastid
INSERT INTO media (user_id, path, mime, size, width, height, color, variants)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetMedia :one
SELECT * FROM media WHERE id = ?;

-- name: ListMediaByIDs :many
SELECT * FROM media WHERE id IN (sqlc.slice(ids));

-- name: UpdateMediaVariants :exec
UPDATE media SET variants = ? WHERE id = ?;

-- name: ListMediaForVariants :many
SELECT id, path, mime, variants FROM media ORDER BY id;
