-- name: ListHiddenBlocks :many
SELECT * FROM post_hidden_blocks WHERE post_id = ? ORDER BY seq;

-- name: AddHiddenBlock :exec
INSERT INTO post_hidden_blocks (post_id, seq, cond, title, content_html) VALUES (?, ?, ?, ?, ?);

-- name: DeleteHiddenBlocks :exec
DELETE FROM post_hidden_blocks WHERE post_id = ?;

-- name: ListResourceLinks :many
SELECT * FROM resource_links WHERE post_id = ? ORDER BY sort, id;

-- name: GetResourceLink :one
SELECT * FROM resource_links WHERE id = ? AND post_id = ?;

-- name: CreateResourceLink :execlastid
INSERT INTO resource_links (post_id, name, url, extract_code, unzip_password, sort, status)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateResourceLink :exec
UPDATE resource_links
SET name = ?, url = ?, extract_code = ?, unzip_password = ?, sort = ?, status = ?
WHERE id = ? AND post_id = ?;

-- name: DeleteResourceLink :exec
DELETE FROM resource_links WHERE id = ? AND post_id = ?;

-- name: DeleteResourceLinks :exec
DELETE FROM resource_links WHERE post_id = ?;

-- name: CreateDownloadLog :exec
INSERT INTO download_logs (user_id, post_id, link_id, is_member_free, ip) VALUES (?, ?, ?, ?, ?);

-- name: CountMemberFreeDownloadsSince :one
SELECT COUNT(DISTINCT post_id) FROM download_logs
WHERE user_id = ? AND is_member_free = 1 AND created_at >= ?;

-- name: HasMemberFreeDownloadSince :one
SELECT EXISTS (
  SELECT 1 FROM download_logs WHERE user_id = ? AND post_id = ? AND is_member_free = 1 AND created_at >= ?
);

-- name: CreateLinkReport :exec
INSERT INTO link_reports (link_id, post_id, user_id, reason) VALUES (?, ?, ?, ?);

-- name: HasPendingLinkReport :one
SELECT EXISTS (SELECT 1 FROM link_reports WHERE link_id = ? AND user_id = ? AND status = 'pending');

-- name: ListLinkReports :many
SELECT r.*, COALESCE(l.name, '') AS link_name, COALESCE(l.url, '') AS link_url, COALESCE(l.status, '') AS link_status,
       COALESCE(p.title, '') AS post_title
FROM link_reports r
LEFT JOIN resource_links l ON l.id = r.link_id
LEFT JOIN posts p ON p.id = r.post_id
WHERE sqlc.narg(status) IS NULL OR r.status = sqlc.narg(status)
ORDER BY r.id DESC
LIMIT ? OFFSET ?;

-- name: CountLinkReports :one
SELECT COUNT(*) FROM link_reports WHERE sqlc.narg(status) IS NULL OR status = sqlc.narg(status);

-- name: GetLinkReport :one
SELECT * FROM link_reports WHERE id = ?;

-- name: UpdateLinkReportStatus :exec
UPDATE link_reports SET status = ?, handled_at = ? WHERE id = ?;

-- name: SetPostCommentCount :exec
UPDATE posts SET comment_count = ? WHERE id = ?;

-- name: ListPostTitles :many
SELECT id, title FROM posts WHERE id IN (sqlc.slice(ids));
