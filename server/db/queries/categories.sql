-- name: ListCategories :many
SELECT * FROM categories ORDER BY sort, id;

-- name: GetCategory :one
SELECT * FROM categories WHERE id = ?;

-- name: CreateCategory :execlastid
INSERT INTO categories (parent_id, name, slug, icon, color, description, sort, seo_title, seo_description)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateCategory :exec
UPDATE categories
SET parent_id = ?, name = ?, slug = ?, icon = ?, color = ?, description = ?, sort = ?, seo_title = ?, seo_description = ?
WHERE id = ?;

-- name: DeleteCategory :exec
DELETE FROM categories WHERE id = ?;

-- name: CountChildCategories :one
SELECT COUNT(*) FROM categories WHERE parent_id = ?;

-- name: CountPostsInCategory :one
SELECT COUNT(*) FROM posts WHERE category_id = ?;

-- name: CountPublishedPostsByCategory :many
SELECT category_id, COUNT(*) AS post_count FROM posts
WHERE status = 'published' AND published_at <= ?
GROUP BY category_id;

-- name: CountActivePostsByCategory :many
SELECT category_id, COUNT(*) AS post_count FROM posts
WHERE status <> 'trash'
GROUP BY category_id;
