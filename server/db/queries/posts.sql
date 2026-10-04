-- name: CreatePost :execlastid
INSERT INTO posts (
  type, title, summary, summary_auto, content_html, toc, cover_media_id, category_id, author_id,
  status, pinned, price, member_free_rank, member_discount, seo_title, seo_keywords, seo_description, published_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdatePost :exec
UPDATE posts
SET type = ?, title = ?, summary = ?, summary_auto = ?, content_html = ?, toc = ?, cover_media_id = ?, category_id = ?,
    status = ?, pinned = ?, price = ?, member_free_rank = ?, member_discount = ?, seo_title = ?, seo_keywords = ?, seo_description = ?, published_at = ?
WHERE id = ?;

-- name: GetPost :one
SELECT * FROM posts WHERE id = ?;

-- name: UpdatePostStatus :execrows
UPDATE posts SET status = sqlc.arg(to_status) WHERE id = ? AND status = sqlc.arg(from_status);

-- name: DeletePost :exec
DELETE FROM posts WHERE id = ?;

-- name: GetResource :one
SELECT * FROM resources WHERE post_id = ?;

-- name: UpsertResource :exec
INSERT INTO resources (post_id, platform, version, file_size, icon_media_id, attrs, changelog, download_cond)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  platform = VALUES(platform), version = VALUES(version), file_size = VALUES(file_size),
  icon_media_id = VALUES(icon_media_id), attrs = VALUES(attrs), changelog = VALUES(changelog),
  download_cond = VALUES(download_cond);

-- name: DeleteResource :exec
DELETE FROM resources WHERE post_id = ?;

-- name: ListPlatformNames :many
SELECT DISTINCT platform FROM resources WHERE platform <> '' ORDER BY platform;
