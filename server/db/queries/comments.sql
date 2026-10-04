-- name: CreateComment :execlastid
INSERT INTO comments (post_id, user_id, parent_id, reply_to_user_id, content, status, ip, user_agent)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetComment :one
SELECT * FROM comments WHERE id = ?;

-- name: UpdateCommentStatus :exec
UPDATE comments SET status = ? WHERE id = ?;

-- name: DeleteComment :exec
DELETE FROM comments WHERE id = ?;

-- name: DeleteCommentReplies :exec
DELETE FROM comments WHERE parent_id = ?;

-- name: CountApprovedComments :one
SELECT COUNT(*) FROM comments WHERE post_id = ? AND status = 'approved';

-- name: HasApprovedComment :one
SELECT EXISTS (SELECT 1 FROM comments WHERE user_id = ? AND post_id = ? AND status = 'approved');

-- name: UserHasApprovedComment :one
SELECT EXISTS (SELECT 1 FROM comments WHERE user_id = ? AND status = 'approved');

-- name: ListRootComments :many
SELECT * FROM comments
WHERE post_id = sqlc.arg(post_id) AND parent_id IS NULL
  AND (status = 'approved' OR (status = 'pending' AND user_id = sqlc.arg(viewer_id)))
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: CountRootComments :one
SELECT COUNT(*) FROM comments
WHERE post_id = sqlc.arg(post_id) AND parent_id IS NULL
  AND (status = 'approved' OR (status = 'pending' AND user_id = sqlc.arg(viewer_id)));

-- name: ListCommentReplies :many
SELECT * FROM comments
WHERE parent_id IN (sqlc.slice(parent_ids))
  AND (status = 'approved' OR (status = 'pending' AND user_id = sqlc.arg(viewer_id)))
ORDER BY id;

-- name: ListCommentsAdmin :many
SELECT * FROM comments
WHERE (sqlc.narg(status) IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(post_id) IS NULL OR post_id = sqlc.narg(post_id))
  AND (sqlc.narg(keyword) IS NULL OR content LIKE sqlc.narg(keyword))
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: CountCommentsAdmin :one
SELECT COUNT(*) FROM comments
WHERE (sqlc.narg(status) IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(post_id) IS NULL OR post_id = sqlc.narg(post_id))
  AND (sqlc.narg(keyword) IS NULL OR content LIKE sqlc.narg(keyword));
