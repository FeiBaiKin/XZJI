package membership

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"xzji/internal/dbgen"
	"xzji/internal/platform/errs"
)

// 套餐价格上限 99999 元
const maxPlanPrice = 99_999_00

var (
	ErrPlanNotFound       = errs.NotFound("PLAN_NOT_FOUND", "套餐不存在或已下架")
	ErrInvalidPlanName    = errs.Invalid("INVALID_NAME", "套餐名称需要 1 到 16 个字")
	ErrInvalidDuration    = errs.Invalid("INVALID_DURATION", "套餐时长需要在 0 到 3650 天之间，0 表示永久")
	ErrInvalidPlanPrice   = errs.Invalid("INVALID_PRICE", "价格需要在 0.01 到 99999 元之间")
	ErrInvalidOrigPrice   = errs.Invalid("INVALID_ORIGINAL_PRICE", "划线价需要高于售价，不显示请填 0")
	ErrInvalidBadge       = errs.Invalid("INVALID_BADGE", "角标不能超过 6 个字")
	ErrLevelHasPlans      = errs.New(errs.KindConflict, "LEVEL_HAS_PLANS", "这个等级下还有套餐，请先删除套餐")
	ErrLevelUnavailable   = errs.Invalid("LEVEL_DISABLED", "这个会员等级已停用")
	ErrPermanentDuplicate = errs.Invalid("PERMANENT_DUPLICATE", "一个等级只能有一个启用的永久套餐")
)

type Plan struct {
	ID            int64
	LevelID       int64
	Name          string
	DurationDays  int
	Price         int64
	OriginalPrice int64
	Badge         string
	Status        string
	Sort          int
}

// Permanent 永久套餐。
func (p Plan) Permanent() bool { return p.DurationDays == 0 }

type PlanInput struct {
	LevelID       int64
	Name          string
	DurationDays  int
	Price         int64
	OriginalPrice int64
	Badge         string
	Status        string
	Sort          int
}

func toPlan(r dbgen.MembershipPlan) Plan {
	return Plan{
		ID: r.ID, LevelID: r.LevelID, Name: r.Name, DurationDays: int(r.DurationDays), Price: r.Price,
		OriginalPrice: r.OriginalPrice, Badge: r.Badge, Status: r.Status, Sort: int(r.Sort),
	}
}

func (s *Service) Plans(ctx context.Context) ([]Plan, error) {
	rows, err := s.q.ListMembershipPlans(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询会员套餐：%w", err)
	}
	out := make([]Plan, len(rows))
	for i, r := range rows {
		out[i] = toPlan(r)
	}
	return out, nil
}

func (s *Service) getPlan(ctx context.Context, id int64) (Plan, error) {
	r, err := s.q.GetMembershipPlan(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Plan{}, ErrPlanNotFound
	}
	if err != nil {
		return Plan{}, fmt.Errorf("查询会员套餐 %d：%w", id, err)
	}
	return toPlan(r), nil
}

func (s *Service) validatePlan(ctx context.Context, id int64, in PlanInput) (PlanInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Badge = strings.TrimSpace(in.Badge)
	if n := utf8.RuneCountInString(in.Name); n == 0 || n > 16 {
		return in, ErrInvalidPlanName
	}
	if in.DurationDays < 0 || in.DurationDays > 3650 {
		return in, ErrInvalidDuration
	}
	if in.Price < 1 || in.Price > maxPlanPrice {
		return in, ErrInvalidPlanPrice
	}
	if in.OriginalPrice != 0 && (in.OriginalPrice <= in.Price || in.OriginalPrice > maxPlanPrice) {
		return in, ErrInvalidOrigPrice
	}
	if utf8.RuneCountInString(in.Badge) > 6 {
		return in, ErrInvalidBadge
	}
	if in.Status == "" {
		in.Status = StatusActive
	}
	if in.Status != StatusActive && in.Status != StatusDisabled {
		return in, ErrInvalidStatus
	}
	if _, err := s.getLevel(ctx, in.LevelID); err != nil {
		return in, err
	}
	// 永久价是永久会员升级时补差价的依据，同一等级只能有一个启用的永久套餐
	if in.DurationDays == 0 && in.Status == StatusActive {
		plans, err := s.Plans(ctx)
		if err != nil {
			return in, err
		}
		for _, p := range plans {
			if p.ID != id && p.LevelID == in.LevelID && p.Permanent() && p.Status == StatusActive {
				return in, ErrPermanentDuplicate
			}
		}
	}
	return in, nil
}

func (s *Service) CreatePlan(ctx context.Context, in PlanInput) (Plan, error) {
	in, err := s.validatePlan(ctx, 0, in)
	if err != nil {
		return Plan{}, err
	}
	id, err := s.q.CreateMembershipPlan(ctx, dbgen.CreateMembershipPlanParams{
		LevelID: in.LevelID, Name: in.Name, DurationDays: int32(in.DurationDays), Price: in.Price,
		OriginalPrice: in.OriginalPrice, Badge: in.Badge, Status: in.Status, Sort: int32(in.Sort),
	})
	if err != nil {
		return Plan{}, fmt.Errorf("新建会员套餐：%w", err)
	}
	return s.getPlan(ctx, id)
}

func (s *Service) UpdatePlan(ctx context.Context, id int64, in PlanInput) (Plan, error) {
	if _, err := s.getPlan(ctx, id); err != nil {
		return Plan{}, err
	}
	in, err := s.validatePlan(ctx, id, in)
	if err != nil {
		return Plan{}, err
	}
	if err := s.q.UpdateMembershipPlan(ctx, dbgen.UpdateMembershipPlanParams{
		LevelID: in.LevelID, Name: in.Name, DurationDays: int32(in.DurationDays), Price: in.Price,
		OriginalPrice: in.OriginalPrice, Badge: in.Badge, Status: in.Status, Sort: int32(in.Sort), ID: id,
	}); err != nil {
		return Plan{}, fmt.Errorf("修改会员套餐 %d：%w", id, err)
	}
	return s.getPlan(ctx, id)
}

// DeletePlan 删除套餐。已有订单保存了下单时的快照，不受影响。
func (s *Service) DeletePlan(ctx context.Context, id int64) error {
	if _, err := s.getPlan(ctx, id); err != nil {
		return err
	}
	if err := s.q.DeleteMembershipPlan(ctx, id); err != nil {
		return fmt.Errorf("删除会员套餐 %d：%w", id, err)
	}
	return nil
}

// LevelPlans 启用的等级及其启用的套餐，用于前台会员页。
type LevelPlans struct {
	Level Level
	Plans []Plan
}

func (s *Service) Catalog(ctx context.Context) ([]LevelPlans, error) {
	levels, err := s.ActiveLevels(ctx)
	if err != nil {
		return nil, err
	}
	plans, err := s.Plans(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LevelPlans, len(levels))
	for i, l := range levels {
		out[i] = LevelPlans{Level: l, Plans: []Plan{}}
		for _, p := range plans {
			if p.LevelID == l.ID && p.Status == StatusActive {
				out[i].Plans = append(out[i].Plans, p)
			}
		}
	}
	return out, nil
}

// PlanQuote 某个用户购买某个套餐的报价。
type PlanQuote struct {
	Plan  Plan
	Level Level
	// From 购买前有效的会员等级，nil 表示不是会员
	From      *Level
	FromUntil *time.Time
	Quote     Quote
}

// QuotePlan 给用户报价，只能购买启用的套餐。
func (s *Service) QuotePlan(ctx context.Context, userID, planID int64) (PlanQuote, error) {
	plan, err := s.getPlan(ctx, planID)
	if err != nil {
		return PlanQuote{}, err
	}
	if plan.Status != StatusActive {
		return PlanQuote{}, ErrPlanNotFound
	}
	m, err := s.Get(ctx, userID)
	if err != nil {
		return PlanQuote{}, err
	}
	return s.quote(ctx, m, plan, true)
}

// quote 按当前会员状态计算；requireActive 为 false 时用于支付后的开通，等级停用也照常开通。
func (s *Service) quote(ctx context.Context, m *Membership, plan Plan, requireActive bool) (PlanQuote, error) {
	level, err := s.getLevel(ctx, plan.LevelID)
	if err != nil {
		return PlanQuote{}, err
	}
	if requireActive && level.Status != StatusActive {
		return PlanQuote{}, ErrLevelUnavailable
	}
	plans, err := s.Plans(ctx)
	if err != nil {
		return PlanQuote{}, err
	}
	of := func(levelID int64) []Plan {
		var out []Plan
		for _, p := range plans {
			if p.LevelID == levelID {
				out = append(out, p)
			}
		}
		return out
	}
	pq := PlanQuote{Plan: plan, Level: level}
	var cur Current
	var curBasis Basis
	if m != nil && m.Active(s.now()) {
		from := m.Level
		pq.From, pq.FromUntil = &from, m.ExpiresAt
		cur = Current{Rank: m.Level.Rank, ExpiresAt: m.ExpiresAt}
		curBasis = BasisOf(of(m.Level.ID))
	}
	pq.Quote, err = Compute(cur, plan, level.Rank, curBasis, BasisOf(of(level.ID)), s.now().UTC())
	return pq, err
}

// ApplyPlanTx 支付成功后在订单的事务里开通会员：锁定会员状态，按当时的状态重新计算到期时间。
// plan 取自订单快照，套餐下单后被修改或删除也按下单时的时长开通。
func (s *Service) ApplyPlanTx(ctx context.Context, tx *sql.Tx, userID int64, plan Plan) (PlanQuote, error) {
	q := s.q.WithTx(tx)
	var m *Membership
	row, err := q.GetUserMembershipForUpdate(ctx, userID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return PlanQuote{}, fmt.Errorf("锁定会员状态：%w", err)
	default:
		lv, err := s.getLevel(ctx, row.LevelID)
		if err != nil && !errors.Is(err, ErrLevelNotFound) {
			return PlanQuote{}, err
		}
		if err == nil {
			m = &Membership{Level: lv, ExpiresAt: ptrTime(row.ExpiresAt)}
		}
	}
	pq, err := s.quote(ctx, m, plan, false)
	if err != nil {
		return pq, err
	}
	var exp sql.NullTime
	if pq.Quote.ExpiresAt != nil {
		exp = sql.NullTime{Time: *pq.Quote.ExpiresAt, Valid: true}
	}
	if err := q.UpsertUserMembership(ctx, dbgen.UpsertUserMembershipParams{UserID: userID, LevelID: plan.LevelID, ExpiresAt: exp}); err != nil {
		return pq, fmt.Errorf("开通会员：%w", err)
	}
	return pq, nil
}

// CleanupExpired 删除已过期的会员状态。判断权限时本来就会校验到期时间，这里只是让会员人数等统计准确。
func (s *Service) CleanupExpired(ctx context.Context) error {
	if _, err := s.q.DeleteExpiredMemberships(ctx, sql.NullTime{Time: s.now().UTC(), Valid: true}); err != nil {
		return fmt.Errorf("清理过期会员：%w", err)
	}
	return nil
}
