package order

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"

	"xzji/internal/dbgen"
)

type Page struct {
	Orders   []Order
	Total    int64
	Page     int
	PageSize int
}

func normPage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return page, size
}

// UserOrders 用户自己的订单，status 为空表示全部。
func (s *Service) UserOrders(ctx context.Context, userID int64, status string, page, size int) (Page, error) {
	page, size = normPage(page, size)
	st := sql.NullString{String: status, Valid: status != ""}
	rows, err := s.q.ListUserOrders(ctx, dbgen.ListUserOrdersParams{UserID: userID, Status: st, Limit: int32(size), Offset: int32((page - 1) * size)})
	if err != nil {
		return Page{}, fmt.Errorf("查询订单：%w", err)
	}
	total, err := s.q.CountUserOrders(ctx, dbgen.CountUserOrdersParams{UserID: userID, Status: st})
	if err != nil {
		return Page{}, fmt.Errorf("统计订单：%w", err)
	}
	return Page{Orders: toOrders(rows), Total: total, Page: page, PageSize: size}, nil
}

// HasPurchased 实现 access.PurchaseChecker。
func (s *Service) HasPurchased(ctx context.Context, userID, postID int64) (bool, error) {
	ok, err := s.q.HasPurchase(ctx, dbgen.HasPurchaseParams{UserID: userID, PostID: postID})
	if err != nil {
		return false, fmt.Errorf("查询购买记录：%w", err)
	}
	return ok, nil
}

type Purchase struct {
	PostID    int64
	Title     string
	OrderNo   string
	Amount    int64
	PayAmount int64
	CreatedAt time.Time
}

type PurchasePage struct {
	Purchases []Purchase
	Total     int64
	Page      int
	PageSize  int
}

// Purchases 用户已购的内容，新的在前。Title 是下单时的标题。
func (s *Service) Purchases(ctx context.Context, userID int64, page, size int) (PurchasePage, error) {
	page, size = normPage(page, size)
	rows, err := s.q.ListUserPurchases(ctx, dbgen.ListUserPurchasesParams{UserID: userID, Limit: int32(size), Offset: int32((page - 1) * size)})
	if err != nil {
		return PurchasePage{}, fmt.Errorf("查询已购内容：%w", err)
	}
	total, err := s.q.CountUserPurchases(ctx, userID)
	if err != nil {
		return PurchasePage{}, fmt.Errorf("统计已购内容：%w", err)
	}
	out := PurchasePage{Purchases: make([]Purchase, len(rows)), Total: total, Page: page, PageSize: size}
	for i, r := range rows {
		out.Purchases[i] = Purchase{PostID: r.PostID, Title: r.Title, OrderNo: r.OrderNo, Amount: r.Amount, PayAmount: r.PayAmount, CreatedAt: r.CreatedAt}
	}
	return out, nil
}

// Stats 用户中心概览：已购数量和累计消费（不含充值）。
func (s *Service) Stats(ctx context.Context, userID int64) (purchases, spent int64, err error) {
	if purchases, err = s.q.CountUserPurchases(ctx, userID); err != nil {
		return 0, 0, fmt.Errorf("统计已购内容：%w", err)
	}
	if spent, err = s.q.SumUserSpent(ctx, userID); err != nil {
		return 0, 0, fmt.Errorf("统计消费金额：%w", err)
	}
	return purchases, spent, nil
}

type Payment struct {
	ID        int64
	Channel   string
	TradeNo   string
	Amount    int64
	Status    string
	Raw       string
	CreatedAt time.Time
}

// Payments 订单的第三方支付记录。
func (s *Service) Payments(ctx context.Context, orderID int64) ([]Payment, error) {
	rows, err := s.q.ListPaymentsOfOrder(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("查询支付记录：%w", err)
	}
	out := make([]Payment, len(rows))
	for i, r := range rows {
		out[i] = Payment{ID: r.ID, Channel: r.Channel, TradeNo: r.TradeNo, Amount: r.Amount, Status: r.Status, Raw: r.RawNotify, CreatedAt: r.CreatedAt}
	}
	return out, nil
}

// AdminFilter 后台订单筛选，零值表示不限。
type AdminFilter struct {
	OrderNo   string
	UserID    int64
	Type      string
	Status    string
	PayMethod string
	// Duplicate 只看有重复支付（需要人工退款）的订单
	Duplicate bool
	Page      int
	PageSize  int
}

var orderColumns = []string{
	"id", "order_no", "user_id", "type", "target_id", "title", "snapshot", "amount", "discount_amount", "pay_amount",
	"pay_method", "status", "expires_at", "paid_at", "refund_method", "refund_remark", "refunded_at", "created_at", "updated_at",
}

// AdminList 后台订单列表。
func (s *Service) AdminList(ctx context.Context, f AdminFilter) (Page, error) {
	page, size := normPage(f.Page, f.PageSize)
	w := sq.And{}
	if f.OrderNo != "" {
		w = append(w, sq.Like{"order_no": f.OrderNo + "%"})
	}
	if f.UserID > 0 {
		w = append(w, sq.Eq{"user_id": f.UserID})
	}
	if f.Type != "" {
		w = append(w, sq.Eq{"type": f.Type})
	}
	if f.Status != "" {
		w = append(w, sq.Eq{"status": f.Status})
	}
	if f.PayMethod != "" {
		w = append(w, sq.Eq{"pay_method": f.PayMethod})
	}
	if f.Duplicate {
		w = append(w, sq.Expr("EXISTS (SELECT 1 FROM payments p WHERE p.order_id = orders.id AND p.status = 'duplicate')"))
	}
	query, args, err := sq.Select(orderColumns...).From("orders").Where(w).OrderBy("id DESC").
		Limit(uint64(size)).Offset(uint64((page - 1) * size)).ToSql()
	if err != nil {
		return Page{}, err
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return Page{}, fmt.Errorf("查询订单：%w", err)
	}
	defer rows.Close()
	var list []dbgen.Order
	for rows.Next() {
		var r dbgen.Order
		if err := rows.Scan(&r.ID, &r.OrderNo, &r.UserID, &r.Type, &r.TargetID, &r.Title, &r.Snapshot, &r.Amount, &r.DiscountAmount, &r.PayAmount,
			&r.PayMethod, &r.Status, &r.ExpiresAt, &r.PaidAt, &r.RefundMethod, &r.RefundRemark, &r.RefundedAt, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return Page{}, fmt.Errorf("读取订单：%w", err)
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return Page{}, fmt.Errorf("读取订单：%w", err)
	}
	out := Page{Orders: toOrders(list), Page: page, PageSize: size}
	countSQL, countArgs, err := sq.Select("COUNT(*)").From("orders").Where(w).ToSql()
	if err != nil {
		return Page{}, err
	}
	if err := s.db.QueryRowContext(ctx, countSQL, countArgs...).Scan(&out.Total); err != nil {
		return Page{}, fmt.Errorf("统计订单：%w", err)
	}
	return out, nil
}
