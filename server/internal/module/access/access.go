// Package access 判定内容的解锁条件和购买价格。判定本身是纯函数，查询访问者身份的部分在 Service 里。
package access

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"xzji/internal/platform/errs"
)

type Kind string

const (
	KindPublic  Kind = "public"
	KindLogin   Kind = "login"
	KindComment Kind = "comment"
	KindPaid    Kind = "paid"
	KindMember  Kind = "member"
)

const MaxRank = 99

// Cond 隐藏块或下载区的解锁条件。Rank 只在 member 条件下使用。
type Cond struct {
	Kind Kind
	Rank int
}

var ErrInvalidCond = errs.Invalid("INVALID_COND", "解锁条件不正确")

// ParseCond 解析 public / login / comment / paid / member:N。
func ParseCond(s string) (Cond, error) {
	switch k := Kind(s); k {
	case KindPublic, KindLogin, KindComment, KindPaid:
		return Cond{Kind: k}, nil
	}
	if rest, ok := strings.CutPrefix(s, string(KindMember)+":"); ok {
		if n, err := strconv.Atoi(rest); err == nil && n >= 1 && n <= MaxRank {
			return Cond{Kind: KindMember, Rank: n}, nil
		}
	}
	return Cond{}, ErrInvalidCond
}

func (c Cond) String() string {
	if c.Kind == KindMember {
		return fmt.Sprintf("%s:%d", KindMember, c.Rank)
	}
	return string(c.Kind)
}

// Rule 内容的付费设置。
type Rule struct {
	// Price 单独购买的价格（分），0 表示不能单独购买
	Price int64
	// MemberFreeRank 会员等级达到该值即免费，0 表示会员也不免费
	MemberFreeRank int
	// MemberDiscount 等级不够免费的会员是否享受等级折扣
	MemberDiscount bool
}

// Viewer 访问者在某篇内容上的身份。
type Viewer struct {
	UserID int64
	// Privileged 管理员或作者本人，始终可见
	Privileged bool
	// MemberRank 当前有效的会员等级，非会员或已过期为 0
	MemberRank   int
	DiscountRate int
	// DailyLimit 每天靠会员身份免费下载的次数，0 表示不限
	DailyLimit int
	Purchased  bool
	// Commented 在这篇内容下有审核通过的评论
	Commented bool
}

// Lock 未解锁时需要的操作。Reason 为 login / comment / paid / member；
// paid 时 Rank 是会员免费等级（0 表示会员也不免费），member 时是要求的等级。
type Lock struct {
	Reason Kind
	Rank   int
}

type Decision struct {
	Allowed bool
	// MemberFree 靠会员身份放行，下载时计入每日次数
	MemberFree bool
	Lock       *Lock
}

// Check 判定访问者能否查看某个条件下的内容。
func Check(c Cond, r Rule, v Viewer) Decision {
	if c.Kind == KindPublic || v.Privileged {
		return Decision{Allowed: true}
	}
	switch c.Kind {
	case KindLogin:
		if v.UserID > 0 {
			return Decision{Allowed: true}
		}
		return Decision{Lock: &Lock{Reason: KindLogin}}
	case KindComment:
		if v.UserID > 0 && v.Commented {
			return Decision{Allowed: true}
		}
		return Decision{Lock: &Lock{Reason: KindComment}}
	case KindPaid:
		if v.UserID > 0 && v.Purchased {
			return Decision{Allowed: true}
		}
		if r.MemberFreeRank > 0 && v.MemberRank >= r.MemberFreeRank {
			return Decision{Allowed: true, MemberFree: true}
		}
		return Decision{Lock: &Lock{Reason: KindPaid, Rank: r.MemberFreeRank}}
	case KindMember:
		if v.MemberRank >= c.Rank {
			return Decision{Allowed: true, MemberFree: true}
		}
		return Decision{Lock: &Lock{Reason: KindMember, Rank: c.Rank}}
	}
	return Decision{Lock: &Lock{Reason: KindLogin}}
}

// Price 访问者单独购买需要支付的金额（分）。ok 为 false 表示不需要或不能单独购买：
// 没有设置价格、已经购买过，或者会员等级已经可以免费查看。
// 等级不够免费的会员按等级折扣计算，四舍五入到分，最低 1 分。
func Price(r Rule, v Viewer) (amount int64, ok bool) {
	if r.Price <= 0 || v.Purchased {
		return 0, false
	}
	if r.MemberFreeRank > 0 && v.MemberRank >= r.MemberFreeRank {
		return 0, false
	}
	amount = r.Price
	if r.MemberDiscount && v.MemberRank > 0 && v.DiscountRate > 0 && v.DiscountRate < 100 {
		amount = (r.Price*int64(v.DiscountRate) + 50) / 100
		amount = max(amount, 1)
	}
	return amount, true
}

// MembershipLookup 查询用户当前有效的会员等级。
type MembershipLookup interface {
	// Active 返回有效会员的等级、折扣和每日下载次数；不是会员时 rank 为 0
	Active(ctx context.Context, userID int64) (rank, discountRate, dailyLimit int, err error)
}

// CommentChecker 查询用户在某篇内容下是否有审核通过的评论。
type CommentChecker interface {
	HasApproved(ctx context.Context, userID, postID int64) (bool, error)
}

// PurchaseChecker 查询用户是否购买过某篇内容，M4 接入订单后实现。
type PurchaseChecker interface {
	HasPurchased(ctx context.Context, userID, postID int64) (bool, error)
}

// Identity 发起请求的用户，UserID 为 0 表示未登录。
type Identity struct {
	UserID int64
	Admin  bool
}

type Service struct {
	members   MembershipLookup
	comments  CommentChecker
	purchases PurchaseChecker
}

// NewService purchases 可以为 nil，此时视为都没有购买。
func NewService(members MembershipLookup, comments CommentChecker, purchases PurchaseChecker) *Service {
	return &Service{members: members, comments: comments, purchases: purchases}
}

// Viewer 查询访问者在某篇内容上的身份。
func (s *Service) Viewer(ctx context.Context, id Identity, postID, authorID int64) (Viewer, error) {
	v := Viewer{UserID: id.UserID, DiscountRate: 100}
	if id.UserID == 0 {
		return v, nil
	}
	v.Privileged = id.Admin || id.UserID == authorID
	var err error
	if v.MemberRank, v.DiscountRate, v.DailyLimit, err = s.members.Active(ctx, id.UserID); err != nil {
		return v, err
	}
	if v.Commented, err = s.comments.HasApproved(ctx, id.UserID, postID); err != nil {
		return v, err
	}
	if s.purchases != nil {
		if v.Purchased, err = s.purchases.HasPurchased(ctx, id.UserID, postID); err != nil {
			return v, err
		}
	}
	return v, nil
}
