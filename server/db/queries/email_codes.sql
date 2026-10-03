-- name: CreateEmailCode :exec
INSERT INTO email_codes (email, purpose, code_hash, ip, expires_at)
VALUES (?, ?, ?, ?, ?);

-- name: GetLatestEmailCode :one
SELECT * FROM email_codes
WHERE email = ? AND purpose = ? AND used_at IS NULL AND expires_at > ?
ORDER BY id DESC
LIMIT 1;

-- name: IncrEmailCodeAttempts :exec
UPDATE email_codes SET attempts = attempts + 1 WHERE id = ?;

-- name: MarkEmailCodeUsed :execrows
UPDATE email_codes SET used_at = ? WHERE id = ? AND used_at IS NULL;

-- name: InvalidateEmailCodes :exec
UPDATE email_codes SET used_at = ? WHERE email = ? AND purpose = ? AND used_at IS NULL;

-- name: DeleteExpiredEmailCodes :execrows
DELETE FROM email_codes WHERE expires_at <= ?;
