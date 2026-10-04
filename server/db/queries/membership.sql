-- name: ListMembershipLevels :many
SELECT l.*, (SELECT COUNT(*) FROM user_memberships m WHERE m.level_id = l.id) AS member_count
FROM membership_levels l
ORDER BY l.`rank`;

-- name: GetMembershipLevel :one
SELECT * FROM membership_levels WHERE id = ?;

-- name: CreateMembershipLevel :execlastid
INSERT INTO membership_levels (name, description, `rank`, discount_rate, daily_download_limit, perks, status)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateMembershipLevel :exec
UPDATE membership_levels
SET name = ?, description = ?, `rank` = ?, discount_rate = ?, daily_download_limit = ?, perks = ?, status = ?
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

-- name: GetUserMembershipForUpdate :one
SELECT * FROM user_memberships WHERE user_id = ? FOR UPDATE;

-- name: DeleteExpiredMemberships :execrows
DELETE FROM user_memberships WHERE expires_at IS NOT NULL AND expires_at < ?;

-- name: ListMembershipPlans :many
SELECT * FROM membership_plans ORDER BY level_id, sort, id;

-- name: GetMembershipPlan :one
SELECT * FROM membership_plans WHERE id = ?;

-- name: CreateMembershipPlan :execlastid
INSERT INTO membership_plans (level_id, name, duration_days, price, original_price, badge, status, sort)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateMembershipPlan :exec
UPDATE membership_plans
SET level_id = ?, name = ?, duration_days = ?, price = ?, original_price = ?, badge = ?, status = ?, sort = ?
WHERE id = ?;

-- name: DeleteMembershipPlan :exec
DELETE FROM membership_plans WHERE id = ?;

-- name: CountPlansOfLevel :one
SELECT COUNT(*) FROM membership_plans WHERE level_id = ?;
