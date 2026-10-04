// Package order 管理订单、第三方支付记录和已购内容。金额一律由服务端计算，支付成功后在同一事务里发放权益。
package order

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"xzji/internal/dbgen"
	"xzji/internal/module/access"
	"xzji/internal/module/content"
	"xzji/internal/module/membership"
	"xzji/internal/module/wallet"
	platformdb "xzji/internal/platform/db"
	"xzji/internal/platform/errs"
	"xzji/internal/platform/ratelimit"
)

type Type string

const (
	TypePost       Type = "post"
	TypeMembership Type = "membership"
	TypeRecharge   Type = "recharge"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusPaid     Status = "paid"
	StatusClosed   Status = "closed"
	StatusRefunded Status = "refunded"
)

// 支付方式；wechat、alipay 同时也是第三方支付记录的渠道名
const (
	MethodWechat  = "wechat"
	MethodAlipay  = "alipay"
	MethodBalance = "balance"
)

// 退款方式
const (
	RefundBalance  = "balance"
	RefundOriginal = "original"
)

const (
	// TTL 订单未支付自动关闭的时间
	TTL = 30 * time.Minute
	// 充值金额范围：1 元到 5000 元
	MinRecharge int64 = 100
	MaxRecharge int64 = 5000_00
	// 每个用户每小时最多下 30 个单，防止刷单
	ordersPerHour = 30
)

var (
	ErrOrderNotFound   = errs.NotFound("ORDER_NOT_FOUND", "订单不存在")
	ErrInvalidType     = errs.Invalid("INVALID_ORDER_TYPE", "订单类型不正确")
	ErrInvalidRecharge = errs.Invalid("INVALID_AMOUNT", "充值金额需要在 1 到 5000 元之间")
	ErrAlreadyBought   = errs.New(errs.KindConflict, "ALREADY_PURCHASED", "你已经购买过了，刷新页面即可查看")
	errRepurchased     = errs.New(errs.KindConflict, "REPURCHASED", "已经购买过这篇内容，本单为重复购买，需要退款")
	ErrNotForSale      = errs.Invalid("NOT_FOR_SALE", "这篇内容不能单独购买")
	ErrMemberFree      = errs.New(errs.KindConflict, "MEMBER_FREE", "你的会员等级可以免费查看，不需要购买")
	ErrNotPending      = errs.New(errs.KindConflict, "ORDER_NOT_PENDING", "订单已支付或已关闭")
	ErrExpired         = errs.New(errs.KindConflict, "ORDER_EXPIRED", "订单已过期，请重新下单")
	ErrBalanceRecharge = errs.Invalid("BALANCE_RECHARGE", "不能用余额充值余额")
	ErrAmountMismatch  = errors.New("回调金额与订单应付金额不一致")
	ErrNotRefundable   = errs.New(errs.KindConflict, "NOT_REFUNDABLE", "只有已支付的订单可以退款")
	ErrRefundMethod    = errs.Invalid("INVALID_REFUND_METHOD", "退款方式不正确")
	ErrRefundDeduct    = errs.New(errs.KindConflict, "REFUND_INSUFFICIENT", "用户余额已不足以扣回这笔充值，请先和用户核实")
	ErrRemarkTooLong   = errs.Invalid("FIELD_TOO_LONG", "备注不能超过 100 个字")
)

// PostSnap 购买内容时的价格依据。
type PostSnap struct {
	PostID         int64 `json:"postId"`
	Price          int64 `json:"price"`
	MemberRank     int   `json:"memberRank"`
	DiscountRate   int   `json:"discountRate"`
	MemberDiscount bool  `json:"memberDiscount"`
}

// MembershipSnap 购买会员时的套餐和折算结果。
type MembershipSnap struct {
	PlanID         int64      `json:"planId"`
	PlanName       string     `json:"planName"`
	LevelID        int64      `json:"levelId"`
	LevelName      string     `json:"levelName"`
	Rank           int        `json:"rank"`
	DurationDays   int        `json:"durationDays"`
	Price          int64      `json:"price"`
	Kind           string     `json:"kind"`
	FromLevel      string     `json:"fromLevel,omitempty"`
	FromExpiresAt  *time.Time `json:"fromExpiresAt,omitempty"`
	FromPermanent  bool       `json:"fromPermanent,omitempty"`
	RemainingDays  int        `json:"remainingDays"`
	RemainingValue int64      `json:"remainingValue"`
	ConvertedDays  int        `json:"convertedDays"`
	Deduct         int64      `json:"deduct"`
	ExpiresAt      *time.Time `json:"expiresAt"`
}

// Grant 支付成功后的发放结果。Error 非空表示没有发放成功，需要人工处理。
type Grant struct {
	At           time.Time  `json:"at"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
	Permanent    bool       `json:"permanent,omitempty"`
	BalanceAfter *int64     `json:"balanceAfter,omitempty"`
	Error        string     `json:"error,omitempty"`
}

type Snapshot struct {
	Post       *PostSnap       `json:"post,omitempty"`
	Membership *MembershipSnap `json:"membership,omitempty"`
	Grant      *Grant          `json:"grant,omitempty"`
}

type Order struct {
	ID             int64
	No             string
	UserID         int64
	Type           Type
	TargetID       int64
	Title          string
	Snapshot       Snapshot
	Amount         int64
	DiscountAmount int64
	PayAmount      int64
	PayMethod      string
	Status         Status
	ExpiresAt      time.Time
	PaidAt         *time.Time
	RefundMethod   string
	RefundRemark   string
	RefundedAt     *time.Time
	CreatedAt      time.Time
}

// Input 下单或报价的参数。TargetID 是内容 ID 或套餐 ID；Amount 只用于充值。
type Input struct {
	UserID   int64
	Admin    bool
	Type     Type
	TargetID int64
	Amount   int64
}

// Quote 下单前的价格明细。
type Quote struct {
	Type           Type
	TargetID       int64
	Title          string
	Amount         int64
	DiscountAmount int64
	PayAmount      int64
	Snapshot       Snapshot
}

// PostLookup 查询可购买的内容，由 content.Service 实现。
type PostLookup interface {
	PurchaseInfo(ctx context.Context, postID int64) (content.PurchaseInfo, error)
}

// ViewerResolver 查询访问者身份，由 access.Service 实现。
type ViewerResolver interface {
	Viewer(ctx context.Context, id access.Identity, postID, authorID int64) (access.Viewer, error)
}

// Memberships 套餐报价与开通，由 membership.Service 实现。
type Memberships interface {
	QuotePlan(ctx context.Context, userID, planID int64) (membership.PlanQuote, error)
	ApplyPlanTx(ctx context.Context, tx *sql.Tx, userID int64, plan membership.Plan) (membership.PlanQuote, error)
}

// Wallets 余额变动，由 wallet.Service 实现。
type Wallets interface {
	ChangeTx(ctx context.Context, tx *sql.Tx, c wallet.Change) (int64, error)
}

// Auditor 记录后台操作日志。
type Auditor interface {
	Record(ctx context.Context, adminID int64, action, targetType string, targetID int64, detail any, ip string) error
}

type Service struct {
	db      *sql.DB
	q       *dbgen.Queries
	posts   PostLookup
	viewers ViewerResolver
	members Memberships
	wallets Wallets
	audit   Auditor
	limiter ratelimit.Limiter
	// loc 订单号里的时间用站点时区
	loc *time.Location
	now func() time.Time
}

func NewService(db *sql.DB, posts PostLookup, members Memberships, wallets Wallets, audit Auditor, limiter ratelimit.Limiter, loc *time.Location) *Service {
	return &Service{db: db, q: dbgen.New(db), posts: posts, members: members, wallets: wallets, audit: audit, limiter: limiter, loc: loc, now: time.Now}
}

// SetViewerResolver access.Service 依赖本模块判断是否已购买，所以装配后再注入。
func (s *Service) SetViewerResolver(v ViewerResolver) { s.viewers = v }

// Quote 计算价格明细，不写数据库。
func (s *Service) Quote(ctx context.Context, in Input) (Quote, error) {
	switch in.Type {
	case TypePost:
		return s.quotePost(ctx, in)
	case TypeMembership:
		return s.quoteMembership(ctx, in)
	case TypeRecharge:
		if in.Amount < MinRecharge || in.Amount > MaxRecharge {
			return Quote{}, ErrInvalidRecharge
		}
		return Quote{Type: TypeRecharge, Title: "余额充值", Amount: in.Amount, PayAmount: in.Amount}, nil
	}
	return Quote{}, ErrInvalidType
}

func (s *Service) quotePost(ctx context.Context, in Input) (Quote, error) {
	info, err := s.posts.PurchaseInfo(ctx, in.TargetID)
	if err != nil {
		return Quote{}, err
	}
	v, err := s.viewers.Viewer(ctx, access.Identity{UserID: in.UserID, Admin: in.Admin}, in.TargetID, info.AuthorID)
	if err != nil {
		return Quote{}, err
	}
	price, ok := access.Price(info.Rule, v)
	if !ok {
		switch {
		case v.Purchased:
			return Quote{}, ErrAlreadyBought
		case info.Rule.Price <= 0:
			return Quote{}, ErrNotForSale
		default:
			return Quote{}, ErrMemberFree
		}
	}
	return Quote{
		Type: TypePost, TargetID: in.TargetID, Title: info.Title,
		Amount: info.Rule.Price, DiscountAmount: info.Rule.Price - price, PayAmount: price,
		Snapshot: Snapshot{Post: &PostSnap{
			PostID: in.TargetID, Price: info.Rule.Price, MemberRank: v.MemberRank, DiscountRate: v.DiscountRate, MemberDiscount: info.Rule.MemberDiscount,
		}},
	}, nil
}

var kindSuffix = map[membership.Kind]string{membership.KindRenew: "（续费）", membership.KindUpgrade: "（升级）"}

func (s *Service) quoteMembership(ctx context.Context, in Input) (Quote, error) {
	pq, err := s.members.QuotePlan(ctx, in.UserID, in.TargetID)
	if err != nil {
		return Quote{}, err
	}
	snap := &MembershipSnap{
		PlanID: pq.Plan.ID, PlanName: pq.Plan.Name, LevelID: pq.Level.ID, LevelName: pq.Level.Name, Rank: pq.Level.Rank,
		DurationDays: pq.Plan.DurationDays, Price: pq.Plan.Price, Kind: string(pq.Quote.Kind),
		RemainingDays: pq.Quote.RemainingDays, RemainingValue: pq.Quote.RemainingValue, ConvertedDays: pq.Quote.ConvertedDays,
		Deduct: pq.Quote.Deduct, ExpiresAt: pq.Quote.ExpiresAt,
	}
	if pq.From != nil {
		snap.FromLevel, snap.FromExpiresAt, snap.FromPermanent = pq.From.Name, pq.FromUntil, pq.FromUntil == nil
	}
	return Quote{
		Type: TypeMembership, TargetID: pq.Plan.ID, Title: pq.Level.Name + " " + pq.Plan.Name + kindSuffix[pq.Quote.Kind],
		Amount: pq.Plan.Price, DiscountAmount: pq.Quote.Deduct, PayAmount: pq.Quote.PayAmount,
		Snapshot: Snapshot{Membership: snap},
	}, nil
}

// newOrderNo 站点时区的年月日时分秒加 6 位随机数，共 20 位。
func (s *Service) newOrderNo() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1_000_000))
	return s.now().In(s.loc).Format("20060102150405") + fmt.Sprintf("%06d", n.Int64())
}

// Create 按服务端计算的价格创建待支付订单。
func (s *Service) Create(ctx context.Context, in Input) (Order, error) {
	q, err := s.Quote(ctx, in)
	if err != nil {
		return Order{}, err
	}
	if n, _ := s.limiter.Incr(fmt.Sprintf("order:create:%d", in.UserID), time.Hour); n > ordersPerHour {
		return Order{}, errs.TooMany("下单太频繁了，请稍后再试")
	}
	snap, err := json.Marshal(q.Snapshot)
	if err != nil {
		return Order{}, err
	}
	expires := s.now().UTC().Add(TTL)
	for range 3 {
		no := s.newOrderNo()
		id, err := s.q.CreateOrder(ctx, dbgen.CreateOrderParams{
			OrderNo: no, UserID: in.UserID, Type: string(q.Type), TargetID: q.TargetID, Title: q.Title, Snapshot: snap,
			Amount: q.Amount, DiscountAmount: q.DiscountAmount, PayAmount: q.PayAmount, ExpiresAt: expires,
		})
		if platformdb.IsDuplicate(err) {
			continue
		}
		if err != nil {
			return Order{}, fmt.Errorf("创建订单：%w", err)
		}
		return s.byID(ctx, id)
	}
	return Order{}, errors.New("生成订单号连续重复")
}

func toOrder(r dbgen.Order) Order {
	o := Order{
		ID: r.ID, No: r.OrderNo, UserID: r.UserID, Type: Type(r.Type), TargetID: r.TargetID, Title: r.Title,
		Amount: r.Amount, DiscountAmount: r.DiscountAmount, PayAmount: r.PayAmount, PayMethod: r.PayMethod, Status: Status(r.Status),
		ExpiresAt: r.ExpiresAt, PaidAt: ptrTime(r.PaidAt), RefundMethod: r.RefundMethod, RefundRemark: r.RefundRemark,
		RefundedAt: ptrTime(r.RefundedAt), CreatedAt: r.CreatedAt,
	}
	_ = json.Unmarshal(r.Snapshot, &o.Snapshot)
	return o
}

func ptrTime(n sql.NullTime) *time.Time {
	if !n.Valid {
		return nil
	}
	v := n.Time
	return &v
}

func (s *Service) byID(ctx context.Context, id int64) (Order, error) {
	r, err := s.q.GetOrder(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrOrderNotFound
	}
	if err != nil {
		return Order{}, fmt.Errorf("查询订单 %d：%w", id, err)
	}
	return toOrder(r), nil
}

// Get 查询订单。userID 不为 0 时只能查自己的订单，别人的订单一律视为不存在。
func (s *Service) Get(ctx context.Context, userID int64, orderNo string) (Order, error) {
	r, err := s.q.GetOrderByNo(ctx, orderNo)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrOrderNotFound
	}
	if err != nil {
		return Order{}, fmt.Errorf("查询订单 %s：%w", orderNo, err)
	}
	if userID != 0 && r.UserID != userID {
		return Order{}, ErrOrderNotFound
	}
	return toOrder(r), nil
}
