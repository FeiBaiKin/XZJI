-- name: ListMembershipLevels :many
SELECT l.*, (SELECT COUNT(*) FROM user_memberships m WHERE m.level_id = l.id) AS member_count
FROM membership_levels l
ORDER BY l.`rank`;

-- name: GetMembershipLevel :one
SELECT * FROM membership_levels WHERE id = ?;

-- name: CreateMembershipLevel :execlastid
INSERT INTO membership_levels (name, `rank`, discount_rate, daily_download_limit, status)
VALUES (?, ?, ?, ?, ?);

-- name: UpdateMembershipLevel :exec
UPDATE membership_levels
SET name = ?, `rank` = ?, discount_rate = ?, daily_download_limit = ?, status = ?
WHERE id = ?;

-- name: DeleteMembershipLevel :exec
DELETE FROM membership_levels WHERE id = ?;

-- name: CountMembersOfLevel :one
SELECT COUNT(*) FROM user_memberships WHERE level_id = ?;

-- name: GetUserMembership :one
SELECT m.user_id, m.level_id, m.expires_at, l.name, l.`rank`, l.discount_rate, l.daily_download_limit
FROM user_memberships m
JOIN membership_levels l ON l.id = m.level_id
WHERE m.user_id = ?;

-- name: ListUserMemberships :many
SELECT m.user_id, m.level_id, m.expires_at, l.name, l.`rank`, l.discount_rate, l.daily_download_limit
FROM user_memberships m
JOIN membership_levels l ON l.id = m.level_id
WHERE m.user_id IN (sqlc.slice(user_ids));

-- name: UpsertUserMembership :exec
INSERT INTO user_memberships (user_id, level_id, expires_at) VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE level_id = VALUES(level_id), expires_at = VALUES(expires_at);

-- name: DeleteUserMembership :exec
DELETE FROM user_memberships WHERE user_id = ?;
