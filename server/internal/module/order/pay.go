package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"xzji/internal/dbgen"
	"xzji/internal/module/membership"
	"xzji/internal/module/wallet"
	platformdb "xzji/internal/platform/db"
	"xzji/internal/platform/errs"
)

// lockOrder 在事务里锁定订单行，同一订单的支付、回调、关闭、退款依次执行。
func lockOrder(ctx context.Context, q *dbgen.Queries, orderNo string) (dbgen.Order, error) {
	r, err := q.GetOrderByNoForUpdate(ctx, orderNo)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrOrderNotFound
	}
	if err != nil {
		return r, fmt.Errorf("锁定订单 %s：%w", orderNo, err)
	}
	return r, nil
}

// PayWithBalance 余额支付：锁定订单 → 锁定钱包、校验余额、写流水、扣余额 → 订单改为已支付 → 发放权益，全部在一个事务里。
// 发放失败（如会员状态在下单后变了）整个事务回滚，不扣钱。
func (s *Service) PayWithBalance(ctx context.Context, userID int64, orderNo string) (Order, error) {
	var id int64
	err := platformdb.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		r, err := lockOrder(ctx, q, orderNo)
		if err != nil {
			return err
		}
		if r.UserID != userID {
			return ErrOrderNotFound
		}
		id = r.ID
		if Status(r.Status) != StatusPending {
			return ErrNotPending
		}
		if Type(r.Type) == TypeRecharge {
			return ErrBalanceRecharge
		}
		now := s.now().UTC()
		if !r.ExpiresAt.After(now) {
			return ErrExpired
		}
		if _, err := s.wallets.ChangeTx(ctx, tx, wallet.Change{
			UserID: userID, Amount: -r.PayAmount, BizType: wallet.BizPurchase, BizID: r.ID, Remark: r.Title,
		}); err != nil {
			return err
		}
		n, err := q.MarkOrderPaid(ctx, dbgen.MarkOrderPaidParams{PayMethod: MethodBalance, PaidAt: sql.NullTime{Time: now, Valid: true}, ID: r.ID})
		if err != nil {
			return fmt.Errorf("更新订单状态：%w", err)
		}
		if n != 1 {
			return ErrNotPending
		}
		return s.grant(ctx, tx, r, true)
	})
	if err != nil {
		return Order{}, err
	}
	return s.byID(ctx, id)
}

// StartPay 记录发起了哪个渠道的支付，补单任务据此查询渠道。
func (s *Service) StartPay(ctx context.Context, orderID int64, method string) error {
	n, err := s.q.StartOrderPay(ctx, dbgen.StartOrderPayParams{PayMethod: method, ID: orderID})
	if err != nil {
		return fmt.Errorf("记录支付方式：%w", err)
	}
	if n != 1 {
		return ErrNotPending
	}
	return nil
}

// Notice 渠道确认到账的通知，来自回调或主动查询。
type Notice struct {
	OrderNo string
	Channel string
	TradeNo string
	Amount  int64
	Raw     string
}

type Outcome int

const (
	// OutcomePaid 本次把订单改为已支付并发放了权益
	OutcomePaid Outcome = iota + 1
	// OutcomeProcessed 同一笔交易已经处理过（重复回调）
	OutcomeProcessed
	// OutcomeDuplicate 订单已由另一笔交易支付，本笔记为重复支付，需要人工退款
	OutcomeDuplicate
)

// ConfirmPaid 处理渠道到账：核对金额 → 订单改为已支付（已超时关闭的也改）→ 写支付记录 → 发放权益。
// 同一笔交易重复通知只处理一次。发放权益遇到业务问题时记录在快照里留给人工处理，不影响确认收款。
func (s *Service) ConfirmPaid(ctx context.Context, n Notice) (Outcome, error) {
	var out Outcome
	err := platformdb.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		r, err := lockOrder(ctx, q, n.OrderNo)
		if err != nil {
			return err
		}
		if n.Amount != r.PayAmount {
			return fmt.Errorf("订单 %s：%w（回调 %d 分，应付 %d 分）", r.OrderNo, ErrAmountMismatch, n.Amount, r.PayAmount)
		}
		if _, err := q.GetPaymentByTrade(ctx, dbgen.GetPaymentByTradeParams{Channel: n.Channel, TradeNo: n.TradeNo}); err == nil {
			out = OutcomeProcessed
			return nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("查询支付记录：%w", err)
		}
		pay := dbgen.CreatePaymentParams{OrderID: r.ID, Channel: n.Channel, TradeNo: n.TradeNo, Amount: n.Amount, Status: "success", RawNotify: n.Raw}
		switch Status(r.Status) {
		case StatusPending, StatusClosed:
			cnt, err := q.MarkOrderPaidByChannel(ctx, dbgen.MarkOrderPaidByChannelParams{
				PayMethod: n.Channel, PaidAt: sql.NullTime{Time: s.now().UTC(), Valid: true}, ID: r.ID,
			})
			if err != nil {
				return fmt.Errorf("更新订单状态：%w", err)
			}
			if cnt != 1 {
				return fmt.Errorf("订单 %s 状态已变化", r.OrderNo)
			}
			if err := q.CreatePayment(ctx, pay); err != nil {
				return fmt.Errorf("写支付记录：%w", err)
			}
			out = OutcomePaid
			return s.grant(ctx, tx, r, false)
		default:
			pay.Status = "duplicate"
			if err := q.CreatePayment(ctx, pay); err != nil {
				return fmt.Errorf("写支付记录：%w", err)
			}
			out = OutcomeDuplicate
			return nil
		}
	})
	return out, err
}

// grant 发放权益并把结果写进快照。strict 为 true 时业务错误直接返回（余额支付整体回滚）；
// 为 false 时（钱已收到）业务错误只记进快照，基础设施错误仍返回让渠道重试。
func (s *Service) grant(ctx context.Context, tx *sql.Tx, r dbgen.Order, strict bool) error {
	q := s.q.WithTx(tx)
	o := toOrder(r)
	g := &Grant{At: s.now().UTC()}
	var err error
	switch o.Type {
	case TypePost:
		var n int64
		n, err = q.AddPurchase(ctx, dbgen.AddPurchaseParams{UserID: o.UserID, PostID: o.TargetID, OrderID: o.ID})
		if err == nil && n == 0 {
			// 同一篇内容下了多张订单：余额支付时整单回滚不扣钱，第三方已到账的留给人工退款
			err = ErrAlreadyBought
			if !strict {
				err = errRepurchased
			}
		}
	case TypeMembership:
		m := o.Snapshot.Membership
		if m == nil {
			return fmt.Errorf("订单 %s 缺少套餐快照", o.No)
		}
		var pq membership.PlanQuote
		pq, err = s.members.ApplyPlanTx(ctx, tx, o.UserID, membership.Plan{ID: m.PlanID, LevelID: m.LevelID, Name: m.PlanName, DurationDays: m.DurationDays, Price: m.Price})
		if err == nil {
			g.ExpiresAt, g.Permanent = pq.Quote.ExpiresAt, pq.Quote.ExpiresAt == nil
		}
	case TypeRecharge:
		var after int64
		after, err = s.wallets.ChangeTx(ctx, tx, wallet.Change{UserID: o.UserID, Amount: o.PayAmount, BizType: wallet.BizRecharge, BizID: o.ID, Remark: "余额充值"})
		g.BalanceAfter = &after
	}
	if err != nil {
		var de *errs.Error
		if strict || !errors.As(err, &de) {
			return err
		}
		g.Error = de.Message
	}
	o.Snapshot.Grant = g
	raw, err := json.Marshal(o.Snapshot)
	if err != nil {
		return err
	}
	if err := q.SetOrderSnapshot(ctx, dbgen.SetOrderSnapshotParams{Snapshot: raw, ID: o.ID}); err != nil {
		return fmt.Errorf("保存发放结果：%w", err)
	}
	return nil
}

// Close 关闭待支付订单，返回是否由本次关闭。
func (s *Service) Close(ctx context.Context, orderID int64) (bool, error) {
	n, err := s.q.CloseOrder(ctx, orderID)
	if err != nil {
		return false, fmt.Errorf("关闭订单 %d：%w", orderID, err)
	}
	return n == 1, nil
}

// ExpiredPending 已过期还没关闭的订单。
func (s *Service) ExpiredPending(ctx context.Context, limit int) ([]Order, error) {
	rows, err := s.q.ListExpiredPendingOrders(ctx, dbgen.ListExpiredPendingOrdersParams{ExpiresAt: s.now().UTC(), Limit: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("查询过期订单：%w", err)
	}
	return toOrders(rows), nil
}

// Awaiting since 之后创建、已发起第三方支付但还没确认到账的订单。
func (s *Service) Awaiting(ctx context.Context, since time.Time, limit int) ([]Order, error) {
	rows, err := s.q.ListAwaitingOrders(ctx, dbgen.ListAwaitingOrdersParams{CreatedAt: since.UTC(), Limit: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("查询待确认订单：%w", err)
	}
	return toOrders(rows), nil
}

func toOrders(rows []dbgen.Order) []Order {
	out := make([]Order, len(rows))
	for i, r := range rows {
		out[i] = toOrder(r)
	}
	return out
}

// Refund 后台退款：订单改为已退款，收回已购内容。method 为 balance 时把应付金额退回余额；
// original 表示管理员已在支付平台原路退回，这里只记录。余额支付的订单只能退回余额；
// 充值订单只能原路退回，同时扣回充值的余额。会员订单不自动扣减时长，由管理员另行调整。
func (s *Service) Refund(ctx context.Context, adminID int64, ip, orderNo, method, remark string) (Order, error) {
	remark = strings.TrimSpace(remark)
	if utf8.RuneCountInString(remark) > 100 {
		return Order{}, ErrRemarkTooLong
	}
	if method != RefundBalance && method != RefundOriginal {
		return Order{}, ErrRefundMethod
	}
	var r dbgen.Order
	err := platformdb.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		var err error
		if r, err = lockOrder(ctx, q, orderNo); err != nil {
			return err
		}
		if Status(r.Status) != StatusPaid {
			return ErrNotRefundable
		}
		switch {
		case r.PayMethod == MethodBalance && method != RefundBalance:
			return errs.Invalid("INVALID_REFUND_METHOD", "余额支付的订单只能退回余额")
		case Type(r.Type) == TypeRecharge && method != RefundOriginal:
			return errs.Invalid("INVALID_REFUND_METHOD", "充值订单只能原路退回，同时扣回充值的余额")
		}
		change := wallet.Change{UserID: r.UserID, Amount: r.PayAmount, BizType: wallet.BizRefund, BizID: r.ID, Remark: "订单退款：" + r.Title}
		if Type(r.Type) == TypeRecharge {
			change.Amount, change.Remark = -r.PayAmount, "充值退款：已原路退回"
		}
		if method == RefundBalance || Type(r.Type) == TypeRecharge {
			if _, err := s.wallets.ChangeTx(ctx, tx, change); err != nil {
				if errors.Is(err, wallet.ErrInsufficient) {
					return ErrRefundDeduct
				}
				return err
			}
		}
		if Type(r.Type) == TypePost {
			if _, err := q.DeletePurchaseOfOrder(ctx, dbgen.DeletePurchaseOfOrderParams{UserID: r.UserID, PostID: r.TargetID, OrderID: r.ID}); err != nil {
				return fmt.Errorf("收回已购内容：%w", err)
			}
		}
		n, err := q.RefundOrder(ctx, dbgen.RefundOrderParams{
			RefundMethod: method, RefundRemark: remark, RefundedAt: sql.NullTime{Time: s.now().UTC(), Valid: true}, ID: r.ID,
		})
		if err != nil {
			return fmt.Errorf("更新订单状态：%w", err)
		}
		if n != 1 {
			return ErrNotRefundable
		}
		return nil
	})
	if err != nil {
		return Order{}, err
	}
	detail := map[string]any{"order_no": r.OrderNo, "amount": r.PayAmount, "method": method, "remark": remark}
	if err := s.audit.Record(ctx, adminID, "order.refund", "order", r.ID, detail, ip); err != nil {
		return Order{}, err
	}
	return s.byID(ctx, r.ID)
}
