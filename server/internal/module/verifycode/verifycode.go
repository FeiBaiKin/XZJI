// Package verifycode 生成、发送和校验邮箱验证码。
package verifycode

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"xzji/internal/dbgen"
	"xzji/internal/platform/errs"
	"xzji/internal/platform/mail"
	"xzji/internal/platform/ratelimit"
)

type Purpose string

const (
	PurposeRegister      Purpose = "register"
	PurposeResetPassword Purpose = "reset_password"
)

const (
	codeTTL     = 5 * time.Minute
	maxAttempts = 5

	resendInterval = time.Minute
	dailyPerEmail  = 10
	dailyPerIP     = 10
)

var (
	ErrInvalidCode = errs.Invalid("INVALID_CODE", "验证码不正确或已过期")
	ErrMailFailed  = errs.New(errs.KindUnavailable, "MAIL_SEND_FAILED", "邮件发送失败，请稍后重试")
	errDailyLimit  = errs.TooMany("今天发送验证码的次数已用完，请明天再试")
)

type Service struct {
	q        dbgen.Querier
	mailer   mail.Sender
	limiter  ratelimit.Limiter
	siteName func(context.Context) string
	log      *slog.Logger
	now      func() time.Time
}

func NewService(q dbgen.Querier, mailer mail.Sender, limiter ratelimit.Limiter, siteName func(context.Context) string, log *slog.Logger) *Service {
	return &Service{q: q, mailer: mailer, limiter: limiter, siteName: siteName, log: log, now: time.Now}
}

// Throttle 检查并计入发送频率：同一邮箱同一用途 60 秒一次；同一邮箱每天 10 次，同一 IP 每天 10 次。
func (s *Service) Throttle(email string, purpose Purpose, ip string) error {
	resendKey := "code:resend:" + string(purpose) + ":" + email
	emailKey, ipKey := "code:email:"+email, "code:ip:"+ip
	if n, retry := s.limiter.Count(resendKey); n > 0 {
		return errs.TooMany(fmt.Sprintf("验证码发送太频繁，请 %d 秒后再试", int(retry.Seconds())+1))
	}
	if n, _ := s.limiter.Count(emailKey); n >= dailyPerEmail {
		return errDailyLimit
	}
	if n, _ := s.limiter.Count(ipKey); n >= dailyPerIP {
		return errDailyLimit
	}
	s.limiter.Incr(resendKey, resendInterval)
	s.limiter.Incr(emailKey, 24*time.Hour)
	s.limiter.Incr(ipKey, 24*time.Hour)
	return nil
}

// Send 生成新验证码并发邮件，同一邮箱同一用途之前未使用的验证码随之作废。email 需已规范化。
func (s *Service) Send(ctx context.Context, email string, purpose Purpose, ip string) error {
	if err := s.Throttle(email, purpose, ip); err != nil {
		return err
	}
	code, err := randomCode()
	if err != nil {
		return err
	}
	now := s.now().UTC()
	if err := s.q.InvalidateEmailCodes(ctx, dbgen.InvalidateEmailCodesParams{
		UsedAt: sql.NullTime{Time: now, Valid: true}, Email: email, Purpose: string(purpose),
	}); err != nil {
		return fmt.Errorf("作废旧验证码：%w", err)
	}
	if err := s.q.CreateEmailCode(ctx, dbgen.CreateEmailCodeParams{
		Email: email, Purpose: string(purpose), CodeHash: hashCode(email, purpose, code), Ip: ip, ExpiresAt: now.Add(codeTTL),
	}); err != nil {
		return fmt.Errorf("保存验证码：%w", err)
	}
	if err := s.mailer.Send(ctx, s.message(ctx, email, purpose, code)); err != nil {
		s.log.Error("发送验证码邮件失败", "email", email, "purpose", purpose, "error", err)
		return ErrMailFailed
	}
	return nil
}

// Verify 校验验证码，成功后立即作废；同一验证码最多允许输错 5 次。
func (s *Service) Verify(ctx context.Context, email string, purpose Purpose, code string) error {
	now := s.now().UTC()
	row, err := s.q.GetLatestEmailCode(ctx, dbgen.GetLatestEmailCodeParams{Email: email, Purpose: string(purpose), ExpiresAt: now})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidCode
	}
	if err != nil {
		return fmt.Errorf("查询验证码：%w", err)
	}
	if row.Attempts >= maxAttempts {
		return ErrInvalidCode
	}
	if subtle.ConstantTimeCompare([]byte(hashCode(email, purpose, code)), []byte(row.CodeHash)) != 1 {
		if err := s.q.IncrEmailCodeAttempts(ctx, row.ID); err != nil {
			return fmt.Errorf("记录验证码错误次数：%w", err)
		}
		return ErrInvalidCode
	}
	n, err := s.q.MarkEmailCodeUsed(ctx, dbgen.MarkEmailCodeUsedParams{UsedAt: sql.NullTime{Time: now, Valid: true}, ID: row.ID})
	if err != nil {
		return fmt.Errorf("作废验证码：%w", err)
	}
	if n == 0 {
		return ErrInvalidCode
	}
	return nil
}

// Cleanup 删除过期验证码，由定时任务调用。
func (s *Service) Cleanup(ctx context.Context) error {
	_, err := s.q.DeleteExpiredEmailCodes(ctx, s.now().UTC().Add(-24*time.Hour))
	return err
}

func (s *Service) message(ctx context.Context, email string, purpose Purpose, code string) mail.Message {
	site := s.siteName(ctx)
	if purpose == PurposeResetPassword {
		return mail.Message{
			To:      email,
			Subject: fmt.Sprintf("【%s】重设密码验证码", site),
			Text:    fmt.Sprintf("你正在重设 %s 的登录密码，验证码是 %s，5 分钟内有效。\n\n如果不是你本人操作，请忽略这封邮件，你的密码不会被修改。", site, code),
		}
	}
	return mail.Message{
		To:      email,
		Subject: fmt.Sprintf("【%s】注册验证码", site),
		Text:    fmt.Sprintf("你的注册验证码是 %s，5 分钟内有效。\n\n如果不是你本人操作，请忽略这封邮件。", code),
	}
}

func randomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func hashCode(email string, purpose Purpose, code string) string {
	sum := sha256.Sum256([]byte(string(purpose) + ":" + email + ":" + code))
	return hex.EncodeToString(sum[:])
}
