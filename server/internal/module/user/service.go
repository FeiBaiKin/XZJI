package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"xzji/internal/dbgen"
	platformdb "xzji/internal/platform/db"
)

// SessionRevoker 让指定用户的登录会话失效，keepSessionID 为空表示全部失效。
type SessionRevoker interface {
	RevokeUser(ctx context.Context, userID int64, keepSessionID string) error
}

// Auditor 记录后台操作日志。
type Auditor interface {
	Record(ctx context.Context, adminID int64, action, targetType string, targetID int64, detail any, ip string) error
}

type Service struct {
	q        dbgen.Querier
	hasher   *Hasher
	sessions SessionRevoker
	audit    Auditor
	now      func() time.Time
	// dummyHash 用于邮箱不存在时也做一次同等耗时的校验，避免通过响应时间判断邮箱是否注册
	dummyHash string
}

func NewService(q dbgen.Querier, hasher *Hasher, sessions SessionRevoker, audit Auditor) (*Service, error) {
	dummy, err := hasher.Hash("dummy-password-for-timing-1")
	if err != nil {
		return nil, err
	}
	return &Service{q: q, hasher: hasher, sessions: sessions, audit: audit, now: time.Now, dummyHash: dummy}, nil
}

func (s *Service) Get(ctx context.Context, id int64) (User, error) {
	row, err := s.q.GetUserByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("查询用户 %d：%w", id, err)
	}
	return fromRow(row), nil
}

// GetByEmail 参数需已规范化。
func (s *Service) GetByEmail(ctx context.Context, email string) (User, error) {
	row, err := s.q.GetUserByEmail(ctx, email)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("按邮箱查询用户：%w", err)
	}
	return fromRow(row), nil
}

type CreateInput struct {
	Email         string
	Password      string
	Nickname      string
	Role          Role
	InviterID     *int64
	EmailVerified bool
}

// Create 新建用户。邀请人不存在或已封禁时忽略邀请关系，不影响注册。
func (s *Service) Create(ctx context.Context, in CreateInput) (User, error) {
	email, err := NormalizeEmail(in.Email)
	if err != nil {
		return User{}, err
	}
	if err := ValidatePassword(in.Password); err != nil {
		return User{}, err
	}
	nickname := DefaultNickname(email)
	if in.Nickname != "" {
		if nickname, err = NormalizeNickname(in.Nickname); err != nil {
			return User{}, err
		}
	}
	role := in.Role
	if role == "" {
		role = RoleUser
	}
	if in.InviterID != nil {
		if inv, err := s.Get(ctx, *in.InviterID); err != nil || inv.Status != StatusActive {
			in.InviterID = nil
		}
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return User{}, err
	}
	var verified sql.NullTime
	if in.EmailVerified {
		verified = sql.NullTime{Time: s.now().UTC(), Valid: true}
	}
	id, err := s.q.CreateUser(ctx, dbgen.CreateUserParams{
		Email:           email,
		PasswordHash:    hash,
		Nickname:        nickname,
		Role:            string(role),
		Status:          string(StatusActive),
		InviterID:       nullInt64(in.InviterID),
		EmailVerifiedAt: verified,
	})
	if platformdb.IsDuplicate(err) {
		return User{}, ErrEmailTaken
	}
	if err != nil {
		return User{}, fmt.Errorf("创建用户：%w", err)
	}
	return s.Get(ctx, id)
}

// VerifyCredentials 校验邮箱和密码。邮箱格式不对、不存在、密码错误都返回同一个错误。
func (s *Service) VerifyCredentials(ctx context.Context, email, password string) (User, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		_, _ = s.hasher.Verify(password, s.dummyHash)
		return User{}, ErrInvalidCredentials
	}
	u, err := s.GetByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		_, _ = s.hasher.Verify(password, s.dummyHash)
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}
	ok, err := s.hasher.Verify(password, u.passwordHash)
	if err != nil {
		return User{}, fmt.Errorf("校验用户 %d 的密码：%w", u.ID, err)
	}
	if !ok {
		return User{}, ErrInvalidCredentials
	}
	if u.Status == StatusBanned {
		return User{}, ErrBanned
	}
	return u, nil
}

func (s *Service) TouchLogin(ctx context.Context, id int64) error {
	return s.q.UpdateUserLastLogin(ctx, dbgen.UpdateUserLastLoginParams{
		LastLoginAt: sql.NullTime{Time: s.now().UTC(), Valid: true},
		ID:          id,
	})
}

// SetPassword 直接设置新密码并让该用户的会话失效，keepSessionID 指定要保留的会话。
func (s *Service) SetPassword(ctx context.Context, id int64, password, keepSessionID string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	if err := s.q.UpdateUserPassword(ctx, dbgen.UpdateUserPasswordParams{PasswordHash: hash, ID: id}); err != nil {
		return fmt.Errorf("更新用户 %d 的密码：%w", id, err)
	}
	return s.sessions.RevokeUser(ctx, id, keepSessionID)
}

// ChangePassword 校验原密码后修改，当前会话保留，其他设备下线。
func (s *Service) ChangePassword(ctx context.Context, id int64, oldPassword, newPassword, currentSessionID string) error {
	u, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	ok, err := s.hasher.Verify(oldPassword, u.passwordHash)
	if err != nil {
		return err
	}
	if !ok {
		return ErrWrongPassword
	}
	return s.SetPassword(ctx, id, newPassword, currentSessionID)
}

func (s *Service) UpdateNickname(ctx context.Context, id int64, nickname string) (User, error) {
	nickname, err := NormalizeNickname(nickname)
	if err != nil {
		return User{}, err
	}
	if err := s.q.UpdateUserNickname(ctx, dbgen.UpdateUserNicknameParams{Nickname: nickname, ID: id}); err != nil {
		return User{}, fmt.Errorf("更新用户 %d 的昵称：%w", id, err)
	}
	return s.Get(ctx, id)
}

type ListFilter struct {
	Keyword  string
	Role     Role
	Status   Status
	Page     int
	PageSize int
}

func (s *Service) List(ctx context.Context, f ListFilter) ([]User, int64, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	var keyword, role, status sql.NullString
	if k := strings.TrimSpace(f.Keyword); k != "" {
		keyword = sql.NullString{String: platformdb.LikeContains(k), Valid: true}
	}
	if f.Role != "" {
		role = sql.NullString{String: string(f.Role), Valid: true}
	}
	if f.Status != "" {
		status = sql.NullString{String: string(f.Status), Valid: true}
	}
	rows, err := s.q.ListUsers(ctx, dbgen.ListUsersParams{
		Keyword: keyword, Role: role, Status: status,
		Limit: int32(f.PageSize), Offset: int32((f.Page - 1) * f.PageSize),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("查询用户列表：%w", err)
	}
	total, err := s.q.CountUsers(ctx, dbgen.CountUsersParams{Keyword: keyword, Role: role, Status: status})
	if err != nil {
		return nil, 0, fmt.Errorf("统计用户数量：%w", err)
	}
	users := make([]User, len(rows))
	for i, r := range rows {
		users[i] = fromRow(r)
	}
	return users, total, nil
}

type AdminChange struct {
	Role   *Role
	Status *Status
}

// Actor 发起后台操作的管理员；ID 为 0 表示命令行操作，不受"不能改自己"限制。
type Actor struct {
	ID int64
	IP string
}

// AdminUpdate 修改用户的角色或状态。封禁后立即让该用户所有会话失效，并写入后台日志。
func (s *Service) AdminUpdate(ctx context.Context, actor Actor, id int64, ch AdminChange) (User, error) {
	if actor.ID != 0 && actor.ID == id {
		return User{}, ErrCannotModifySelf
	}
	u, err := s.Get(ctx, id)
	if err != nil {
		return User{}, err
	}
	role, status := u.Role, u.Status
	if ch.Role != nil {
		if *ch.Role != RoleUser && *ch.Role != RoleAdmin {
			return User{}, ErrInvalidRole
		}
		role = *ch.Role
	}
	if ch.Status != nil {
		if *ch.Status != StatusActive && *ch.Status != StatusBanned {
			return User{}, ErrInvalidStatus
		}
		status = *ch.Status
	}
	if role == u.Role && status == u.Status {
		return u, nil
	}
	if err := s.q.UpdateUserRoleStatus(ctx, dbgen.UpdateUserRoleStatusParams{Role: string(role), Status: string(status), ID: id}); err != nil {
		return User{}, fmt.Errorf("更新用户 %d 的角色和状态：%w", id, err)
	}
	if status == StatusBanned && u.Status != StatusBanned {
		if err := s.sessions.RevokeUser(ctx, id, ""); err != nil {
			return User{}, err
		}
	}
	detail := map[string]any{
		"before": map[string]string{"role": string(u.Role), "status": string(u.Status)},
		"after":  map[string]string{"role": string(role), "status": string(status)},
	}
	if err := s.audit.Record(ctx, actor.ID, "user.update", "user", id, detail, actor.IP); err != nil {
		return User{}, err
	}
	u.Role, u.Status = role, status
	return u, nil
}

// Nicknames 批量查询昵称，用于展示内容作者。不存在的 ID 不出现在结果里。
func (s *Service) Nicknames(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.q.ListUserNicknames(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("批量查询昵称：%w", err)
	}
	for _, r := range rows {
		out[r.ID] = r.Nickname
	}
	return out, nil
}

// Exists 用户不存在时返回 ErrNotFound。
func (s *Service) Exists(ctx context.Context, id int64) error {
	_, err := s.Get(ctx, id)
	return err
}

// Brief 用户的简要信息，用于后台订单、流水列表。
type Brief struct {
	ID       int64
	Nickname string
	Email    string
}

// Briefs 批量查询昵称和邮箱，不存在的 ID 不出现在结果里。
func (s *Service) Briefs(ctx context.Context, ids []int64) (map[int64]Brief, error) {
	out := make(map[int64]Brief, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.q.ListUserBriefs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("批量查询用户：%w", err)
	}
	for _, r := range rows {
		out[r.ID] = Brief{ID: r.ID, Nickname: r.Nickname, Email: r.Email}
	}
	return out, nil
}
