// Package session 管理登录会话。客户端只持有随机令牌，数据库只存令牌的 SHA-256，
// 即使数据库泄露也无法直接冒用会话。
package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"xzji/internal/dbgen"
)

type Client string

const (
	ClientWeb   Client = "web"
	ClientAdmin Client = "admin"
	ClientApp   Client = "app"
)

const (
	// TTLRemember 勾选"30 天内自动登录"和 App 的会话时长，访问时自动续期
	TTLRemember = 30 * 24 * time.Hour
	// TTLShort 未勾选自动登录时的会话时长，浏览器端 Cookie 关闭浏览器即失效
	TTLShort = 24 * time.Hour
	// TTLAdmin 后台会话时长，不续期
	TTLAdmin = 12 * time.Hour
)

var ErrNotFound = errors.New("会话不存在或已过期")

type Session struct {
	ID        string
	UserID    int64
	Client    Client
	TTL       time.Duration
	ExpiresAt time.Time
	Role      string
	Status    string
}

type Service struct {
	q   dbgen.Querier
	now func() time.Time
}

func NewService(q dbgen.Querier) *Service {
	return &Service{q: q, now: time.Now}
}

// HashToken 计算令牌对应的会话 ID。
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Create 创建会话，返回给客户端的原始令牌只在这里出现一次。
func (s *Service) Create(ctx context.Context, userID int64, client Client, ttl time.Duration, ip, userAgent string) (string, Session, error) {
	token, err := newToken()
	if err != nil {
		return "", Session{}, err
	}
	sess := Session{
		ID:        HashToken(token),
		UserID:    userID,
		Client:    client,
		TTL:       ttl,
		ExpiresAt: s.now().UTC().Add(ttl),
	}
	err = s.q.CreateSession(ctx, dbgen.CreateSessionParams{
		ID:         sess.ID,
		UserID:     userID,
		Client:     string(client),
		TtlSeconds: int32(ttl / time.Second),
		Ip:         ip,
		UserAgent:  truncate(userAgent, 255),
		ExpiresAt:  sess.ExpiresAt,
	})
	if err != nil {
		return "", Session{}, fmt.Errorf("创建会话：%w", err)
	}
	return token, sess, nil
}

// Lookup 按令牌查找未过期的会话，同时带出用户当前的角色和状态。
func (s *Service) Lookup(ctx context.Context, token string) (Session, error) {
	if token == "" {
		return Session{}, ErrNotFound
	}
	row, err := s.q.GetActiveSession(ctx, dbgen.GetActiveSessionParams{ID: HashToken(token), ExpiresAt: s.now().UTC()})
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("查询会话：%w", err)
	}
	return Session{
		ID:        row.ID,
		UserID:    row.UserID,
		Client:    Client(row.Client),
		TTL:       time.Duration(row.TtlSeconds) * time.Second,
		ExpiresAt: row.ExpiresAt,
		Role:      row.Role,
		Status:    row.Status,
	}, nil
}

// Renew 剩余时间不足一半时续期，避免每个请求都写数据库。后台会话不续期。
func (s *Service) Renew(ctx context.Context, sess Session) (Session, bool, error) {
	if sess.Client == ClientAdmin {
		return sess, false, nil
	}
	now := s.now().UTC()
	if sess.ExpiresAt.Sub(now) > sess.TTL/2 {
		return sess, false, nil
	}
	sess.ExpiresAt = now.Add(sess.TTL)
	if err := s.q.ExtendSession(ctx, dbgen.ExtendSessionParams{ExpiresAt: sess.ExpiresAt, ID: sess.ID}); err != nil {
		return sess, false, fmt.Errorf("续期会话：%w", err)
	}
	return sess, true, nil
}

func (s *Service) Revoke(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.q.DeleteSession(ctx, HashToken(token))
}

// RevokeUser 让用户的会话失效，keepSessionID 为空表示全部失效。
func (s *Service) RevokeUser(ctx context.Context, userID int64, keepSessionID string) error {
	if err := s.q.DeleteUserSessionsExcept(ctx, dbgen.DeleteUserSessionsExceptParams{UserID: userID, ID: keepSessionID}); err != nil {
		return fmt.Errorf("注销用户 %d 的会话：%w", userID, err)
	}
	return nil
}

// Cleanup 删除过期会话，由定时任务调用。
func (s *Service) Cleanup(ctx context.Context) error {
	_, err := s.q.DeleteExpiredSessions(ctx, s.now().UTC())
	return err
}

func truncate(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	s = s[:maxBytes]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
