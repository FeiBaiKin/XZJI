package membership

import (
	"time"

	"xzji/internal/platform/errs"
)

const day = 24 * time.Hour

// Kind 购买套餐的方式。
type Kind string

const (
	KindNew     Kind = "new"
	KindRenew   Kind = "renew"
	KindUpgrade Kind = "upgrade"
)

var (
	ErrDowngrade        = errs.Invalid("PLAN_DOWNGRADE", "会员有效期内不能购买比当前等级低的套餐")
	ErrAlreadyPermanent = errs.Invalid("ALREADY_PERMANENT", "你已经是这个等级的永久会员了")
	ErrPermanentOnly    = errs.Invalid("PERMANENT_ONLY", "永久会员只能升级到更高等级的永久套餐")
)

// Basis 计算等级日均价和永久差价的依据。
type Basis struct {
	// Price ÷ Days 为日均价；Days 为 0 表示没有有限期套餐，剩余价值按 0 计
	Price int64
	Days  int
	// PermanentPrice 永久套餐的价格，0 表示没有永久套餐
	PermanentPrice int64
}

// BasisOf 从一个等级的套餐里选出计算依据：日均价取 365 天的年卡，没有年卡取时长最长的有限期套餐；
// 永久价取永久套餐。启用的套餐优先，同类里按排序靠前的为准；没有启用的同类套餐时才用停用的。
func BasisOf(plans []Plan) Basis {
	pick := func(active bool, ok func(Plan) bool) (Plan, bool) {
		var best Plan
		found := false
		for _, p := range plans {
			if (p.Status == StatusActive) == active && ok(p) && (!found || better(p, best)) {
				best, found = p, true
			}
		}
		return best, found
	}
	var b Basis
	for _, active := range []bool{true, false} {
		p, ok := pick(active, func(p Plan) bool { return p.DurationDays == 365 })
		if !ok {
			p, ok = pick(active, func(p Plan) bool { return p.DurationDays > 0 })
		}
		if ok {
			b.Price, b.Days = p.Price, p.DurationDays
			break
		}
	}
	for _, active := range []bool{true, false} {
		if p, ok := pick(active, Plan.Permanent); ok {
			b.PermanentPrice = p.Price
			break
		}
	}
	return b
}

// better 有限期套餐时长更长的优先，其次按排序和 ID。
func better(p, cur Plan) bool {
	if p.DurationDays != cur.DurationDays && !p.Permanent() && !cur.Permanent() {
		return p.DurationDays > cur.DurationDays
	}
	if p.Sort != cur.Sort {
		return p.Sort < cur.Sort
	}
	return p.ID < cur.ID
}

// Current 用户当前的会员状态；Rank 为 0 表示不是会员或已经过期。
type Current struct {
	Rank int
	// ExpiresAt 为 nil 表示永久
	ExpiresAt *time.Time
}

// Quote 购买某个套餐的价格和开通结果。
type Quote struct {
	Kind Kind
	// Price 套餐价；Deduct 升级到永久套餐时抵扣的金额
	Price     int64
	Deduct    int64
	PayAmount int64
	// RemainingDays 和 RemainingValue 升级时原等级剩余的天数和按日均价折算的价值
	RemainingDays  int
	RemainingValue int64
	// ConvertedDays 升级到有限期套餐时，剩余价值折算成新等级的天数
	ConvertedDays int
	// ExpiresAt 开通后的到期时间，nil 表示永久
	ExpiresAt *time.Time
}

// Compute 计算购买套餐的价格和开通结果。规则：
//   - 不是会员或已过期：从现在开通；
//   - 同等级续费有限期套餐：在原到期时间上叠加；
//   - 升级（更高等级，或同等级从有限期升到永久）：剩余价值 = 剩余整天数 × 原等级日均价（四舍五入到分）；
//     升到有限期套餐时折算成新等级的天数（向下取整）叠加到套餐时长上，升到永久套餐时直接抵扣价格；
//   - 永久会员只能升级到更高等级的永久套餐，补两者永久价的差价；
//   - 有效期内不能买比当前等级低的套餐。
//
// 抵扣后最少支付 1 分。curBasis 是当前等级的计算依据，newBasis 是目标等级的。
func Compute(cur Current, plan Plan, rank int, curBasis, newBasis Basis, now time.Time) (Quote, error) {
	q := Quote{Kind: KindNew, Price: plan.Price, PayAmount: plan.Price}
	expires := func(from time.Time, days int) *time.Time {
		t := from.Add(time.Duration(days) * day)
		return &t
	}
	if cur.Rank == 0 {
		if !plan.Permanent() {
			q.ExpiresAt = expires(now, plan.DurationDays)
		}
		return q, nil
	}
	if rank < cur.Rank {
		return q, ErrDowngrade
	}

	if cur.ExpiresAt == nil {
		if rank == cur.Rank {
			return q, ErrAlreadyPermanent
		}
		if !plan.Permanent() {
			return q, ErrPermanentOnly
		}
		q.Kind = KindUpgrade
		q.PayAmount = max(plan.Price-curBasis.PermanentPrice, 1)
		q.Deduct = plan.Price - q.PayAmount
		return q, nil
	}

	if rank == cur.Rank && !plan.Permanent() {
		q.Kind = KindRenew
		q.ExpiresAt = expires(*cur.ExpiresAt, plan.DurationDays)
		return q, nil
	}

	q.Kind = KindUpgrade
	q.RemainingDays = max(int(cur.ExpiresAt.Sub(now)/day), 0)
	if curBasis.Days > 0 {
		q.RemainingValue = (int64(q.RemainingDays)*curBasis.Price + int64(curBasis.Days)/2) / int64(curBasis.Days)
	}
	if plan.Permanent() {
		q.PayAmount = max(plan.Price-q.RemainingValue, 1)
		q.Deduct = plan.Price - q.PayAmount
		return q, nil
	}
	if newBasis.Days > 0 && newBasis.Price > 0 {
		q.ConvertedDays = int(q.RemainingValue * int64(newBasis.Days) / newBasis.Price)
	}
	q.ExpiresAt = expires(now, plan.DurationDays+q.ConvertedDays)
	return q, nil
}
