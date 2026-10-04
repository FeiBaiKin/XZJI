-- name: CreateOrder :execlastid
INSERT INTO orders (order_no, user_id, type, target_id, title, snapshot, amount, discount_amount, pay_amount, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetOrder :one
SELECT * FROM orders WHERE id = ?;

-- name: GetOrderByNo :one
SELECT * FROM orders WHERE order_no = ?;

-- name: GetOrderByNoForUpdate :one
SELECT * FROM orders WHERE order_no = ? FOR UPDATE;

-- name: StartOrderPay :execrows
UPDATE orders SET pay_method = ? WHERE id = ? AND status = 'pending';

-- name: MarkOrderPaid :execrows
UPDATE orders SET status = 'paid', pay_method = ?, paid_at = ? WHERE id = ? AND status = 'pending';

-- name: MarkOrderPaidByChannel :execrows
-- 渠道确认到账时，已超时关闭的订单同样改为已支付：钱已经收到，不能不认
UPDATE orders SET status = 'paid', pay_method = ?, paid_at = ? WHERE id = ? AND status IN ('pending', 'closed');

-- name: SetOrderSnapshot :exec
UPDATE orders SET snapshot = ? WHERE id = ?;

-- name: CloseOrder :execrows
UPDATE orders SET status = 'closed' WHERE id = ? AND status = 'pending';

-- name: RefundOrder :execrows
UPDATE orders SET status = 'refunded', refund_method = ?, refund_remark = ?, refunded_at = ? WHERE id = ? AND status = 'paid';

-- name: ListExpiredPendingOrders :many
SELECT * FROM orders WHERE status = 'pending' AND expires_at <= ? ORDER BY id LIMIT ?;

-- name: ListAwaitingOrders :many
-- 已发起第三方支付、还没确认到账的订单，用于补单
SELECT * FROM orders
WHERE status = 'pending' AND pay_method IN ('wechat', 'alipay') AND created_at >= ?
ORDER BY id LIMIT ?;

-- name: ListUserOrders :many
SELECT * FROM orders
WHERE user_id = sqlc.arg(user_id) AND (sqlc.narg(status) IS NULL OR status = sqlc.narg(status))
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: CountUserOrders :one
SELECT COUNT(*) FROM orders
WHERE user_id = sqlc.arg(user_id) AND (sqlc.narg(status) IS NULL OR status = sqlc.narg(status));

-- name: SumUserSpent :one
SELECT CAST(COALESCE(SUM(pay_amount), 0) AS SIGNED) FROM orders WHERE user_id = ? AND status = 'paid' AND type <> 'recharge';

-- name: CreatePayment :exec
INSERT INTO payments (order_id, channel, trade_no, amount, status, raw_notify) VALUES (?, ?, ?, ?, ?, ?);

-- name: GetPaymentByTrade :one
SELECT * FROM payments WHERE channel = ? AND trade_no = ?;

-- name: ListPaymentsOfOrder :many
SELECT * FROM payments WHERE order_id = ? ORDER BY id;

-- name: AddPurchase :execrows
INSERT IGNORE INTO user_purchases (user_id, post_id, order_id) VALUES (?, ?, ?);

-- name: HasPurchase :one
SELECT EXISTS (SELECT 1 FROM user_purchases WHERE user_id = ? AND post_id = ?);

-- name: DeletePurchaseOfOrder :execrows
DELETE FROM user_purchases WHERE user_id = ? AND post_id = ? AND order_id = ?;

-- name: ListUserPurchases :many
SELECT up.post_id, up.created_at, o.order_no, o.title, o.pay_amount, o.amount
FROM user_purchases up
JOIN orders o ON o.id = up.order_id
WHERE up.user_id = ?
ORDER BY up.created_at DESC
LIMIT ? OFFSET ?;

-- name: CountUserPurchases :one
SELECT COUNT(*) FROM user_purchases WHERE user_id = ?;
