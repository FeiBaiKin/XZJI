-- name: GetTag :one
SELECT * FROM tags WHERE id = ?;

-- name: GetTagBySlug :one
SELECT * FROM tags WHERE slug = ?;

-- name: ListTagsByNames :many
SELECT * FROM tags WHERE name IN (sqlc.slice(names));

-- name: CreateTag :execlastid
INSERT INTO tags (name, slug) VALUES (?, ?);

-- name: UpdateTag :exec
UPDATE tags SET name = ?, slug = ? WHERE id = ?;

-- name: DeleteTag :exec
DELETE FROM tags WHERE id = ?;

-- name: DeletePostTagsByTag :exec
DELETE FROM post_tags WHERE tag_id = ?;

-- name: ListTagsWithCount :many
SELECT t.id, t.name, t.slug, t.created_at, COUNT(pt.post_id) AS post_count
FROM tags t
LEFT JOIN post_tags pt ON pt.tag_id = t.id
WHERE sqlc.narg(keyword) IS NULL OR t.name LIKE sqlc.narg(keyword) OR t.slug LIKE sqlc.narg(keyword)
GROUP BY t.id, t.name, t.slug, t.created_at
ORDER BY t.id DESC
LIMIT ? OFFSET ?;

-- name: CountTags :one
SELECT COUNT(*) FROM tags
WHERE sqlc.narg(keyword) IS NULL OR name LIKE sqlc.narg(keyword) OR slug LIKE sqlc.narg(keyword);

-- name: ListHotTags :many
SELECT t.id, t.name, t.slug, COUNT(*) AS post_count
FROM tags t
JOIN post_tags pt ON pt.tag_id = t.id
JOIN posts p ON p.id = pt.post_id
WHERE p.status = 'published' AND p.published_at <= ?
GROUP BY t.id, t.name, t.slug
ORDER BY post_count DESC, t.id DESC
LIMIT ?;

-- name: CountPublishedPostsByTag :one
SELECT COUNT(*) FROM post_tags pt
JOIN posts p ON p.id = pt.post_id
WHERE pt.tag_id = ? AND p.status = 'published' AND p.published_at <= ?;

-- name: ListTagsByPostIDs :many
SELECT pt.post_id, t.id, t.name, t.slug
FROM post_tags pt
JOIN tags t ON t.id = pt.tag_id
WHERE pt.post_id IN (sqlc.slice(post_ids))
ORDER BY pt.post_id, pt.sort;

-- name: AddPostTag :exec
INSERT INTO post_tags (post_id, tag_id, sort) VALUES (?, ?, ?);

-- name: DeletePostTags :exec
DELETE FROM post_tags WHERE post_id = ?;

-- name: GetTagByName :one
SELECT * FROM tags WHERE name = ?;
