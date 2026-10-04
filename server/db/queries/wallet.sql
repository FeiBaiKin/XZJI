-- name: EnsureWallet :exec
-- 用 ON DUPLICATE KEY UPDATE 直接拿行的排他锁；INSERT IGNORE 遇到已有行只拿共享锁，
-- 随后 FOR UPDATE 升级为排他锁时，并发的两个事务会互相死锁
INSERT INTO wallets (user_id, balance) VALUES (?, 0) ON DUPLICATE KEY UPDATE balance = balance;

-- name: GetWalletBalance :one
SELECT balance FROM wallets WHERE user_id = ?;

-- name: GetWalletForUpdate :one
SELECT balance FROM wallets WHERE user_id = ? FOR UPDATE;

-- name: SetWalletBalance :exec
UPDATE wallets SET balance = ? WHERE user_id = ?;

-- name: ListWalletBalances :many
SELECT user_id, balance FROM wallets WHERE user_id IN (sqlc.slice(user_ids));

-- name: CreateLedgerEntry :exec
INSERT INTO wallet_ledger (user_id, account, amount, balance_after, biz_type, biz_id, remark) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListUserLedger :many
SELECT * FROM wallet_ledger WHERE user_id = ? AND account = ? ORDER BY id DESC LIMIT ? OFFSET ?;

-- name: CountUserLedger :one
SELECT COUNT(*) FROM wallet_ledger WHERE user_id = ? AND account = ?;
