package authn

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"

	"xzji/internal/httpapi/apierr"
)

const (
	HeaderCSRF   = "X-CSRF-Token"
	csrfTokenLen = 43 // 32 字节的 base64url 编码长度
	csrfMaxAge   = 7 * 24 * time.Hour
)

// CSRF 双重提交校验：令牌同时存在于 HttpOnly Cookie 和请求头，两者一致才放行。
// 攻击页面拿不到令牌（跨域读不到 GET /auth/csrf 的响应），也就伪造不出请求头。
type CSRF struct {
	Cookies Cookies
	// AllowOrigin 判断带 Origin 的写请求来源是否可信
	AllowOrigin func(origin string, r *http.Request) bool
}

// Issue 返回当前有效的令牌，没有时生成并写入 Cookie。
func (x *CSRF) Issue(c *gin.Context) string {
	if v, err := c.Cookie(x.Cookies.CSRF); err == nil && len(v) == csrfTokenLen {
		return v
	}
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	token := base64.RawURLEncoding.EncodeToString(b)
	http.SetCookie(c.Writer, &http.Cookie{
		Name: x.Cookies.CSRF, Value: token, Path: "/", Domain: x.Cookies.Domain,
		MaxAge: int(csrfMaxAge / time.Second), Expires: time.Now().Add(csrfMaxAge),
		HttpOnly: true, Secure: x.Cookies.Secure, SameSite: x.Cookies.SameSite,
	})
	return token
}

// Check 校验写请求。没有 Origin 且不带任何 Cookie 的请求视为非浏览器客户端（App、小程序），
// 它们用 Bearer 令牌认证，不存在 CSRF 风险。
func (x *CSRF) Check(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return
	}
	origin := c.GetHeader("Origin")
	_, sessErr := c.Cookie(x.Cookies.Session)
	cookieToken, csrfErr := c.Cookie(x.Cookies.CSRF)
	if origin == "" && sessErr != nil && csrfErr != nil {
		return
	}
	if origin != "" && !x.AllowOrigin(origin, c.Request) {
		apierr.Write(c, apierr.CSRFOrigin)
		return
	}
	header := c.GetHeader(HeaderCSRF)
	if header == "" || cookieToken == "" || subtle.ConstantTimeCompare([]byte(header), []byte(cookieToken)) != 1 {
		apierr.Write(c, apierr.CSRFInvalid)
	}
}

// OriginIn 来源必须在白名单内。
func OriginIn(origins []string) func(string, *http.Request) bool {
	allowed := make(map[string]bool, len(origins))
	for _, o := range origins {
		allowed[o] = true
	}
	return func(origin string, _ *http.Request) bool { return allowed[origin] }
}

// SameHost 来源必须与请求的 Host 相同，用于只允许同源访问的后台。
func SameHost(origin string, r *http.Request) bool {
	u, err := url.Parse(origin)
	return err == nil && u.Host != "" && u.Host == r.Host
}
