-- name: ListPublishedPostUpdates :many
-- sitemap 用：已发布内容的 ID 和更新时间，定时未到的不算
SELECT id, updated_at FROM posts
WHERE status = 'published' AND published_at <= sqlc.arg(at)
ORDER BY id;

-- name: ListCategorySlugs :many
SELECT slug FROM categories ORDER BY sort, id;

-- name: ListTagSlugsWithPosts :many
-- 只有还有已发布内容的标签才进 sitemap
SELECT DISTINCT t.slug FROM tags t
JOIN post_tags pt ON pt.tag_id = t.id
JOIN posts p ON p.id = pt.post_id
WHERE p.status = 'published' AND p.published_at <= sqlc.arg(at)
ORDER BY t.slug;
