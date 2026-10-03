package authn

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCSRFCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	x := &CSRF{
		Cookies:     Cookies{Session: "s", CSRF: "c"},
		AllowOrigin: OriginIn([]string{"http://www.test"}),
	}
	token := strings.Repeat("t", csrfTokenLen)

	tests := []struct {
		name     string
		method   string
		origin   string
		cookies  map[string]string
		header   string
		wantCode string
	}{
		{name: "GET 不校验", method: http.MethodGet, origin: "http://evil.test"},
		{name: "无来源无 Cookie 的客户端放行", method: http.MethodPost},
		{name: "可信来源缺少令牌", method: http.MethodPost, origin: "http://www.test", wantCode: "CSRF_INVALID"},
		{name: "不可信来源", method: http.MethodPost, origin: "http://evil.test", cookies: map[string]string{"c": token}, header: token, wantCode: "CSRF_ORIGIN"},
		{name: "令牌一致", method: http.MethodPost, origin: "http://www.test", cookies: map[string]string{"c": token}, header: token},
		{name: "令牌不一致", method: http.MethodPost, origin: "http://www.test", cookies: map[string]string{"c": token}, header: strings.Repeat("x", csrfTokenLen), wantCode: "CSRF_INVALID"},
		{name: "带会话 Cookie 但无来源也要校验", method: http.MethodPost, cookies: map[string]string{"s": "sess"}, wantCode: "CSRF_INVALID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(tt.method, "/x", nil)
			if tt.origin != "" {
				c.Request.Header.Set("Origin", tt.origin)
			}
			for k, v := range tt.cookies {
				c.Request.AddCookie(&http.Cookie{Name: k, Value: v})
			}
			if tt.header != "" {
				c.Request.Header.Set(HeaderCSRF, tt.header)
			}
			x.Check(c)
			if tt.wantCode == "" {
				if c.IsAborted() {
					t.Fatalf("不应拦截，响应：%s", w.Body.String())
				}
				return
			}
			if !c.IsAborted() || w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), tt.wantCode) {
				t.Fatalf("应返回 403 %s，得到 %d %s", tt.wantCode, w.Code, w.Body.String())
			}
		})
	}
}

func TestSameHost(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "http://admin.test/x", nil)
	if !SameHost("https://admin.test", r) {
		t.Fatal("同主机应通过")
	}
	if SameHost("https://www.test", r) || SameHost("null", r) {
		t.Fatal("不同主机应拒绝")
	}
}
