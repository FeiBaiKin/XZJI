-- name: CreateAdminLog :exec
INSERT INTO admin_logs (admin_id, action, target_type, target_id, detail, ip)
VALUES (?, ?, ?, ?, ?, ?);
