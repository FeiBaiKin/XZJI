// Package authn 识别请求身份：从 Cookie 或 Bearer 令牌加载会话，并负责 CSRF 防护。
// 前台和后台各用一个 Realm，Cookie 名称和允许的会话类型都不同，两边会话互不通用。
package authn

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"xzji/internal/module/session"
	"xzji/internal/module/user"
)

const ctxIdentity = "authn.identity"

// Identity 当前请求的登录身份。
type Identity struct {
	UserID    int64
	Role      user.Role
	SessionID string
	Token     string
	Client    session.Client
}

// FromContext 取出当前请求的身份。strict handler 收到的 ctx 就是 *gin.Context。
func FromContext(ctx context.Context) (*Identity, bool) {
	id, ok := ctx.Value(ctxIdentity).(*Identity)
	return id, ok
}

// Cookies 一个 Realm 使用的 Cookie 设置。
type Cookies struct {
	Session  string
	CSRF     string
	Domain   string
	Secure   bool
	SameSite http.SameSite
}

// SetSession 写入会话 Cookie；maxAge 为 0 时是浏览器会话 Cookie，关闭浏览器即失效。
func (k Cookies) SetSession(c *gin.Context, token string, maxAge time.Duration) {
	ck := &http.Cookie{
		Name: k.Session, Value: token, Path: "/", Domain: k.Domain,
		HttpOnly: true, Secure: k.Secure, SameSite: k.SameSite,
	}
	if maxAge > 0 {
		ck.MaxAge = int(maxAge / time.Second)
		ck.Expires = time.Now().Add(maxAge)
	}
	http.SetCookie(c.Writer, ck)
}

func (k Cookies) ClearSession(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: k.Session, Value: "", Path: "/", Domain: k.Domain, MaxAge: -1,
		HttpOnly: true, Secure: k.Secure, SameSite: k.SameSite,
	})
}

// Realm 一组身份识别规则。
type Realm struct {
	Cookies  Cookies
	Clients  map[session.Client]bool
	Bearer   bool
	Role     user.Role // 非空时要求用户具备该角色
	Sessions *session.Service
	Log      *slog.Logger
}

// Token 取出请求携带的令牌：优先 Cookie，允许时再看 Authorization: Bearer。
func (r *Realm) Token(c *gin.Context) (token string, fromCookie bool) {
	if v, err := c.Cookie(r.Cookies.Session); err == nil && v != "" {
		return v, true
	}
	if r.Bearer {
		if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
			return strings.TrimSpace(h[len("Bearer "):]), false
		}
	}
	return "", false
}

// Load 加载身份；没有令牌或令牌无效时不报错，由需要登录的接口自行拒绝。
// 会话剩余时间不足一半时续期，并刷新持久 Cookie 的有效期。
func (r *Realm) Load(c *gin.Context) {
	token, fromCookie := r.Token(c)
	if token == "" {
		return
	}
	sess, err := r.Sessions.Lookup(c, token)
	if err != nil {
		if !errors.Is(err, session.ErrNotFound) {
			r.Log.Error("加载会话失败", "error", err)
		}
		if fromCookie {
			r.Cookies.ClearSession(c)
		}
		return
	}
	if !r.Clients[sess.Client] || sess.Status != string(user.StatusActive) || (r.Role != "" && sess.Role != string(r.Role)) {
		if fromCookie {
			r.Cookies.ClearSession(c)
		}
		return
	}
	if renewed, ok, err := r.Sessions.Renew(c, sess); err != nil {
		r.Log.Error("续期会话失败", "error", err)
	} else if ok && fromCookie && sess.TTL > session.TTLShort {
		r.Cookies.SetSession(c, token, renewed.TTL)
	}
	c.Set(ctxIdentity, &Identity{
		UserID: sess.UserID, Role: user.Role(sess.Role), SessionID: sess.ID, Token: token, Client: sess.Client,
	})
}
