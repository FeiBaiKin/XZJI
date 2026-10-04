// Package membership 管理会员等级和用户的会员状态。套餐售卖和开通订单在 M4 接入。
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
	platformdb "xzji/internal/platform/db"
	"xzji/internal/platform/errs"
)

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
	maxRank        = 99
)

var (
	ErrLevelNotFound   = errs.NotFound("LEVEL_NOT_FOUND", "会员等级不存在")
	ErrRankTaken       = errs.New(errs.KindConflict, "RANK_TAKEN", "已经有相同数字的等级了")
	ErrLevelInUse      = errs.New(errs.KindConflict, "LEVEL_IN_USE", "还有用户是这个等级的会员，不能删除，可以改为停用")
	ErrInvalidLevel    = errs.Invalid("INVALID_LEVEL", "等级数字需要在 1 到 99 之间")
	ErrInvalidDiscount = errs.Invalid("INVALID_DISCOUNT", "折扣需要在 1 到 100 之间，100 表示不打折")
	ErrInvalidLimit    = errs.Invalid("INVALID_LIMIT", "每日下载次数不能小于 0")
	ErrInvalidName     = errs.Invalid("INVALID_NAME", "等级名称需要 1 到 16 个字")
	ErrInvalidStatus   = errs.Invalid("INVALID_STATUS", "状态不正确")
	ErrExpired         = errs.Invalid("INVALID_EXPIRES", "到期时间需要晚于现在")
)

type Level struct {
	ID                 int64
	Name               string
	Rank               int
	DiscountRate       int
	DailyDownloadLimit int
	Status             string
	MemberCount        int64
}

// Membership 用户的会员状态；ExpiresAt 为 nil 表示永久。
type Membership struct {
	Level     Level
	ExpiresAt *time.Time
}

// Active 是否在有效期内。
func (m Membership) Active(now time.Time) bool {
	return m.ExpiresAt == nil || m.ExpiresAt.After(now)
}

type LevelInput struct {
	Name               string
	Rank               int
	DiscountRate       int
	DailyDownloadLimit int
	Status             string
}

// Auditor 记录后台操作日志。
type Auditor interface {
	Record(ctx context.Context, adminID int64, action, targetType string, targetID int64, detail any, ip string) error
}

// UserChecker 确认用户存在。
type UserChecker interface {
	Exists(ctx context.Context, id int64) error
}

type Service struct {
	q     *dbgen.Queries
	users UserChecker
	audit Auditor
	now   func() time.Time
}

func NewService(q *dbgen.Queries, users UserChecker, audit Auditor) *Service {
	return &Service{q: q, users: users, audit: audit, now: time.Now}
}

func (s *Service) Levels(ctx context.Context) ([]Level, error) {
	rows, err := s.q.ListMembershipLevels(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询会员等级：%w", err)
	}
	out := make([]Level, len(rows))
	for i, r := range rows {
		out[i] = Level{
			ID: r.ID, Name: r.Name, Rank: int(r.Rank), DiscountRate: int(r.DiscountRate),
			DailyDownloadLimit: int(r.DailyDownloadLimit), Status: r.Status, MemberCount: r.MemberCount,
		}
	}
	return out, nil
}

// ActiveLevels 前台展示用，只含启用的等级。
func (s *Service) ActiveLevels(ctx context.Context) ([]Level, error) {
	all, err := s.Levels(ctx)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, l := range all {
		if l.Status == StatusActive {
			out = append(out, l)
		}
	}
	return out, nil
}

func (s *Service) getLevel(ctx context.Context, id int64) (Level, error) {
	r, err := s.q.GetMembershipLevel(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Level{}, ErrLevelNotFound
	}
	if err != nil {
		return Level{}, fmt.Errorf("查询会员等级 %d：%w", id, err)
	}
	return Level{ID: r.ID, Name: r.Name, Rank: int(r.Rank), DiscountRate: int(r.DiscountRate), DailyDownloadLimit: int(r.DailyDownloadLimit), Status: r.Status}, nil
}

func validateLevel(in LevelInput) (LevelInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	if n := utf8.RuneCountInString(in.Name); n == 0 || n > 16 {
		return in, ErrInvalidName
	}
	if in.Rank < 1 || in.Rank > maxRank {
		return in, ErrInvalidLevel
	}
	if in.DiscountRate < 1 || in.DiscountRate > 100 {
		return in, ErrInvalidDiscount
	}
	if in.DailyDownloadLimit < 0 {
		return in, ErrInvalidLimit
	}
	if in.Status == "" {
		in.Status = StatusActive
	}
	if in.Status != StatusActive && in.Status != StatusDisabled {
		return in, ErrInvalidStatus
	}
	return in, nil
}

func (s *Service) CreateLevel(ctx context.Context, in LevelInput) (Level, error) {
	in, err := validateLevel(in)
	if err != nil {
		return Level{}, err
	}
	id, err := s.q.CreateMembershipLevel(ctx, dbgen.CreateMembershipLevelParams{
		Name: in.Name, Rank: int32(in.Rank), DiscountRate: int32(in.DiscountRate), DailyDownloadLimit: int32(in.DailyDownloadLimit), Status: in.Status,
	})
	if platformdb.IsDuplicate(err) {
		return Level{}, ErrRankTaken
	}
	if err != nil {
		return Level{}, fmt.Errorf("新建会员等级：%w", err)
	}
	return s.getLevel(ctx, id)
}

func (s *Service) UpdateLevel(ctx context.Context, id int64, in LevelInput) (Level, error) {
	if _, err := s.getLevel(ctx, id); err != nil {
		return Level{}, err
	}
	in, err := validateLevel(in)
	if err != nil {
		return Level{}, err
	}
	err = s.q.UpdateMembershipLevel(ctx, dbgen.UpdateMembershipLevelParams{
		Name: in.Name, Rank: int32(in.Rank), DiscountRate: int32(in.DiscountRate), DailyDownloadLimit: int32(in.DailyDownloadLimit), Status: in.Status, ID: id,
	})
	if platformdb.IsDuplicate(err) {
		return Level{}, ErrRankTaken
	}
	if err != nil {
		return Level{}, fmt.Errorf("修改会员等级 %d：%w", id, err)
	}
	return s.getLevel(ctx, id)
}

func (s *Service) DeleteLevel(ctx context.Context, id int64) error {
	if _, err := s.getLevel(ctx, id); err != nil {
		return err
	}
	n, err := s.q.CountMembersOfLevel(ctx, id)
	if err != nil {
		return fmt.Errorf("统计等级会员数：%w", err)
	}
	if n > 0 {
		return ErrLevelInUse
	}
	if err := s.q.DeleteMembershipLevel(ctx, id); err != nil {
		return fmt.Errorf("删除会员等级 %d：%w", id, err)
	}
	return nil
}

func ptrTime(n sql.NullTime) *time.Time {
	if !n.Valid {
		return nil
	}
	t := n.Time
	return &t
}

// Get 查询用户的会员状态，包括已过期的；不是会员时返回 nil。
func (s *Service) Get(ctx context.Context, userID int64) (*Membership, error) {
	r, err := s.q.GetUserMembership(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询用户 %d 的会员状态：%w", userID, err)
	}
	return &Membership{
		Level:     Level{ID: r.LevelID, Name: r.Name, Rank: int(r.Rank), DiscountRate: int(r.DiscountRate), DailyDownloadLimit: int(r.DailyDownloadLimit)},
		ExpiresAt: ptrTime(r.ExpiresAt),
	}, nil
}

// ForUsers 批量查询会员状态（包括已过期的），用于后台用户列表和评论区的会员标识。
func (s *Service) ForUsers(ctx context.Context, ids []int64) (map[int64]Membership, error) {
	out := make(map[int64]Membership, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.q.ListUserMemberships(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("批量查询会员状态：%w", err)
	}
	for _, r := range rows {
		out[r.UserID] = Membership{
			Level:     Level{ID: r.LevelID, Name: r.Name, Rank: int(r.Rank), DiscountRate: int(r.DiscountRate), DailyDownloadLimit: int(r.DailyDownloadLimit)},
			ExpiresAt: ptrTime(r.ExpiresAt),
		}
	}
	return out, nil
}

// Active 实现 access.MembershipLookup：返回有效会员的等级权益，非会员或已过期时 rank 为 0。
// 每次判定都校验到期时间，不依赖定时任务是否及时处理到期。
func (s *Service) Active(ctx context.Context, userID int64) (rank, discountRate, dailyLimit int, err error) {
	m, err := s.Get(ctx, userID)
	if err != nil {
		return 0, 100, 0, err
	}
	if m == nil || !m.Active(s.now()) {
		return 0, 100, 0, nil
	}
	return m.Level.Rank, m.Level.DiscountRate, m.Level.DailyDownloadLimit, nil
}

// Grant 后台手动开通或修改会员，expiresAt 为 nil 表示永久。
func (s *Service) Grant(ctx context.Context, adminID int64, ip string, userID, levelID int64, expiresAt *time.Time) (*Membership, error) {
	if err := s.users.Exists(ctx, userID); err != nil {
		return nil, err
	}
	level, err := s.getLevel(ctx, levelID)
	if err != nil {
		return nil, err
	}
	var exp sql.NullTime
	if expiresAt != nil {
		if !expiresAt.After(s.now()) {
			return nil, ErrExpired
		}
		exp = sql.NullTime{Time: expiresAt.UTC(), Valid: true}
	}
	if err := s.q.UpsertUserMembership(ctx, dbgen.UpsertUserMembershipParams{UserID: userID, LevelID: levelID, ExpiresAt: exp}); err != nil {
		return nil, fmt.Errorf("开通会员：%w", err)
	}
	detail := map[string]any{"level": level.Name, "rank": level.Rank, "expires_at": expiresAt}
	if err := s.audit.Record(ctx, adminID, "membership.grant", "user", userID, detail, ip); err != nil {
		return nil, err
	}
	return s.Get(ctx, userID)
}

// Revoke 后台取消会员。
func (s *Service) Revoke(ctx context.Context, adminID int64, ip string, userID int64) error {
	if err := s.q.DeleteUserMembership(ctx, userID); err != nil {
		return fmt.Errorf("取消会员：%w", err)
	}
	return s.audit.Record(ctx, adminID, "membership.revoke", "user", userID, nil, ip)
}

// Badges 有效会员的等级名称，用于评论区的会员标识。
func (s *Service) Badges(ctx context.Context, ids []int64) (map[int64]string, error) {
	ms, err := s.ForUsers(ctx, ids)
	if err != nil {
		return nil, err
	}
	now := s.now()
	out := make(map[int64]string, len(ms))
	for id, m := range ms {
		if m.Active(now) {
			out[id] = m.Level.Name
		}
	}
	return out, nil
}
