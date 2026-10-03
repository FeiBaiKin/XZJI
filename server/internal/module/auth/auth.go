// Package auth 编排注册、登录、找回密码等认证流程，组合用户、会话和验证码模块。
package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"xzji/internal/module/session"
	"xzji/internal/module/user"
	"xzji/internal/module/verifycode"
	"xzji/internal/platform/errs"
	"xzji/internal/platform/ratelimit"
)

const (
	loginWindow       = 15 * time.Minute
	loginFailsPerIP   = 20
	loginFailsPerAcct = 5
	registerPerIPHour = 10
)

// Meta 请求的来源信息，用于限流和会话记录。
type Meta struct {
	IP        string
	UserAgent string
}

// Result 登录或注册成功后的用户与会话。
type Result struct {
	User    user.User
	Token   string
	Session session.Session
}

type Service struct {
	users    *user.Service
	sessions *session.Service
	codes    *verifycode.Service
	limiter  ratelimit.Limiter
}

func NewService(users *user.Service, sessions *session.Service, codes *verifycode.Service, limiter ratelimit.Limiter) *Service {
	return &Service{users: users, sessions: sessions, codes: codes, limiter: limiter}
}

// SendCode 发送验证码。注册时邮箱已存在直接提示；找回密码时邮箱不存在也按成功处理，但同样计入频率限制。
func (s *Service) SendCode(ctx context.Context, email string, purpose verifycode.Purpose, meta Meta) error {
	email, err := user.NormalizeEmail(email)
	if err != nil {
		return err
	}
	_, err = s.users.GetByEmail(ctx, email)
	exists := err == nil
	if err != nil && !errors.Is(err, user.ErrNotFound) {
		return err
	}
	switch purpose {
	case verifycode.PurposeRegister:
		if exists {
			return user.ErrEmailTaken
		}
	case verifycode.PurposeResetPassword:
		if !exists {
			return s.codes.Throttle(email, purpose, meta.IP)
		}
	default:
		return errs.Invalid("INVALID_PURPOSE", "验证码用途不正确")
	}
	return s.codes.Send(ctx, email, purpose, meta.IP)
}

type RegisterInput struct {
	Email     string
	Code      string
	Password  string
	Nickname  string
	InviterID *int64
	Client    session.Client
}

// Register 校验验证码后创建用户并直接登录。先校验密码和邮箱占用，避免白白消耗验证码。
func (s *Service) Register(ctx context.Context, in RegisterInput, meta Meta) (Result, error) {
	if n, _ := s.limiter.Incr("register:ip:"+meta.IP, time.Hour); n > registerPerIPHour {
		return Result{}, errs.TooMany("注册太频繁，请稍后再试")
	}
	email, err := user.NormalizeEmail(in.Email)
	if err != nil {
		return Result{}, err
	}
	if err := user.ValidatePassword(in.Password); err != nil {
		return Result{}, err
	}
	if in.Nickname != "" {
		if _, err := user.NormalizeNickname(in.Nickname); err != nil {
			return Result{}, err
		}
	}
	if _, err := s.users.GetByEmail(ctx, email); err == nil {
		return Result{}, user.ErrEmailTaken
	} else if !errors.Is(err, user.ErrNotFound) {
		return Result{}, err
	}
	if err := s.codes.Verify(ctx, email, verifycode.PurposeRegister, in.Code); err != nil {
		return Result{}, err
	}
	u, err := s.users.Create(ctx, user.CreateInput{
		Email: email, Password: in.Password, Nickname: in.Nickname, InviterID: in.InviterID, EmailVerified: true,
	})
	if err != nil {
		return Result{}, err
	}
	return s.startSession(ctx, u, in.Client, session.TTLRemember, meta)
}

type LoginInput struct {
	Email    string
	Password string
	Remember bool
	Client   session.Client
}

// Login 邮箱密码登录。同一 IP 15 分钟内失败 20 次、同一账号失败 5 次后暂时拒绝。
func (s *Service) Login(ctx context.Context, in LoginInput, meta Meta) (Result, error) {
	u, err := s.checkCredentials(ctx, "login", in.Email, in.Password, meta)
	if err != nil {
		return Result{}, err
	}
	ttl := session.TTLShort
	if in.Remember || in.Client == session.ClientApp {
		ttl = session.TTLRemember
	}
	return s.startSession(ctx, u, in.Client, ttl, meta)
}

// AdminLogin 后台登录，只允许管理员。非管理员与密码错误返回同样的提示，不暴露账号角色。
func (s *Service) AdminLogin(ctx context.Context, email, password string, meta Meta) (Result, error) {
	u, err := s.checkCredentials(ctx, "admin-login", email, password, meta)
	if err != nil {
		return Result{}, err
	}
	if u.Role != user.RoleAdmin {
		s.recordFailure("admin-login", email, meta)
		return Result{}, user.ErrInvalidCredentials
	}
	return s.startSession(ctx, u, session.ClientAdmin, session.TTLAdmin, meta)
}

func (s *Service) checkCredentials(ctx context.Context, scope, email, password string, meta Meta) (user.User, error) {
	ipKey, acctKey := failKeys(scope, email, meta)
	if n, retry := s.limiter.Count(acctKey); n >= loginFailsPerAcct {
		return user.User{}, tooManyFails(retry)
	}
	if n, retry := s.limiter.Count(ipKey); n >= loginFailsPerIP {
		return user.User{}, tooManyFails(retry)
	}
	u, err := s.users.VerifyCredentials(ctx, email, password)
	if errors.Is(err, user.ErrInvalidCredentials) {
		s.recordFailure(scope, email, meta)
		return user.User{}, err
	}
	if err != nil {
		return user.User{}, err
	}
	s.limiter.Reset(acctKey)
	return u, nil
}

func (s *Service) recordFailure(scope, email string, meta Meta) {
	ipKey, acctKey := failKeys(scope, email, meta)
	s.limiter.Incr(ipKey, loginWindow)
	s.limiter.Incr(acctKey, loginWindow)
}

func failKeys(scope, email string, meta Meta) (string, string) {
	if e, err := user.NormalizeEmail(email); err == nil {
		email = e
	}
	return scope + ":ip:" + meta.IP, scope + ":acct:" + email
}

func tooManyFails(retry time.Duration) error {
	minutes := int(retry.Minutes()) + 1
	return errs.TooMany(fmt.Sprintf("登录失败次数太多，请 %d 分钟后再试", minutes))
}

func (s *Service) startSession(ctx context.Context, u user.User, client session.Client, ttl time.Duration, meta Meta) (Result, error) {
	if client == "" {
		client = session.ClientWeb
	}
	token, sess, err := s.sessions.Create(ctx, u.ID, client, ttl, meta.IP, meta.UserAgent)
	if err != nil {
		return Result{}, err
	}
	if err := s.users.TouchLogin(ctx, u.ID); err != nil {
		return Result{}, err
	}
	return Result{User: u, Token: token, Session: sess}, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.sessions.Revoke(ctx, token)
}

// ResetPassword 用邮箱验证码重设密码，成功后该账号所有会话失效。
func (s *Service) ResetPassword(ctx context.Context, email, code, password string) error {
	email, err := user.NormalizeEmail(email)
	if err != nil {
		return err
	}
	if err := user.ValidatePassword(password); err != nil {
		return err
	}
	if err := s.codes.Verify(ctx, email, verifycode.PurposeResetPassword, code); err != nil {
		return err
	}
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return err
	}
	return s.users.SetPassword(ctx, u.ID, password, "")
}
