package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"xzji/internal/config"
	"xzji/internal/module/user"
	"xzji/internal/platform/mail"
	"xzji/internal/testutil"
)

const testOrigin = "http://www.test"

type fakeMailer struct {
	mu   sync.Mutex
	sent map[string][]mail.Message
}

func (f *fakeMailer) Send(_ context.Context, m mail.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent[m.To] = append(f.sent[m.To], m)
	return nil
}

var codeRe = regexp.MustCompile(`\b\d{6}\b`)

func (f *fakeMailer) lastCode(t *testing.T, to string) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	msgs := f.sent[to]
	if len(msgs) == 0 {
		t.Fatalf("%s 没有收到邮件", to)
	}
	return codeRe.FindString(msgs[len(msgs)-1].Text)
}

func (f *fakeMailer) count(to string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent[to])
}

type env struct {
	db     *sql.DB
	srv    *httptest.Server
	svc    *Services
	mailer *fakeMailer
}

// newEnv 装配一套完整的服务，可选的 opts 用来改配置（例如把清除通知指向假的“前台服务器”）。
func newEnv(t *testing.T, opts ...func(*config.Config)) *env {
	t.Helper()
	db := testutil.DB(t)
	log := slog.New(slog.NewTextHandler(testLog{t}, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &config.Config{
		Env:      config.EnvDev,
		HTTP:     config.HTTP{TrustedProxies: []string{"127.0.0.1"}},
		CORS:     config.CORS{AllowOrigins: []string{testOrigin}},
		Media:    config.Media{Dir: t.TempDir(), BaseURL: "http://img.test", MaxSizeMB: 2},
		Timezone: "Asia/Shanghai",
		Pay:      config.Pay{Mock: true},

		SecretKeyFile: filepath.Join(t.TempDir(), "secret.key"),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	mailer := &fakeMailer{sent: map[string][]mail.Message{}}
	svc, err := NewServices(db, mailer, log, cfg)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRouter(cfg, db, log, svc)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return &env{db: db, srv: srv, svc: svc, mailer: mailer}
}

// browser 模拟浏览器：自动带 Cookie 和 Origin，写请求前先取 CSRF 令牌。
type browser struct {
	t      *testing.T
	base   string
	http   *http.Client
	csrf   map[string]string
	origin string
}

// testLog 把服务端错误日志写进测试输出，只在测试失败或 -v 时显示。
type testLog struct{ t *testing.T }

func (w testLog) Write(p []byte) (int, error) {
	w.t.Log(strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

func (e *env) browser(t *testing.T) *browser {
	jar, _ := cookiejar.New(nil)
	return &browser{t: t, base: e.srv.URL, http: &http.Client{Jar: jar}, csrf: map[string]string{}, origin: testOrigin}
}

type resp struct {
	status int
	body   map[string]any
	header http.Header
}

func (r resp) code() string {
	if e, ok := r.body["error"].(map[string]any); ok {
		s, _ := e["code"].(string)
		return s
	}
	return ""
}

func (b *browser) do(method, path string, body any, withCSRF bool, header map[string]string) resp {
	b.t.Helper()
	var rd io.Reader
	h := map[string]string{}
	if body != nil {
		buf, _ := json.Marshal(body)
		rd = bytes.NewReader(buf)
		h["Content-Type"] = "application/json"
	}
	for k, v := range header {
		h[k] = v
	}
	return b.send(method, path, rd, withCSRF, h)
}

func (b *browser) send(method, path string, body io.Reader, withCSRF bool, header map[string]string) resp {
	b.t.Helper()
	req, _ := http.NewRequest(method, b.base+path, body)
	if b.origin != "" {
		req.Header.Set("Origin", b.origin)
	}
	if withCSRF {
		prefix := "/api/v1"
		if len(path) > 10 && path[:10] == "/admin-api" {
			prefix = "/admin-api/v1"
		}
		if b.csrf[prefix] == "" {
			r := b.do(http.MethodGet, prefix+"/auth/csrf", nil, false, nil)
			b.csrf[prefix], _ = r.body["token"].(string)
		}
		req.Header.Set("X-CSRF-Token", b.csrf[prefix])
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	res, err := b.http.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	defer res.Body.Close()
	out := resp{status: res.StatusCode, body: map[string]any{}, header: res.Header}
	raw, _ := io.ReadAll(res.Body)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.body)
	}
	return out
}

func (b *browser) post(path string, body any) resp {
	return b.do(http.MethodPost, path, body, true, nil)
}
func (b *browser) get(path string) resp { return b.do(http.MethodGet, path, nil, false, nil) }

func (b *browser) cookie(name string) string {
	u, _ := url.Parse(b.base)
	for _, c := range b.http.Jar.Cookies(u) {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

func (b *browser) setCookie(name, value string) {
	u, _ := url.Parse(b.base)
	b.http.Jar.SetCookies(u, []*http.Cookie{{Name: name, Value: value, Path: "/"}})
}

func expect(t *testing.T, r resp, status int, code string) {
	t.Helper()
	if r.status != status || r.code() != code {
		t.Fatalf("期望 %d %q，得到 %d %q %v", status, code, r.status, r.code(), r.body)
	}
}

func (e *env) register(t *testing.T, b *browser, email, password string) {
	t.Helper()
	expect(t, b.post("/api/v1/auth/email-codes", map[string]any{"email": email, "purpose": "register"}), 204, "")
	code := e.mailer.lastCode(t, email)
	expect(t, b.post("/api/v1/auth/register", map[string]any{"email": email, "code": code, "password": password}), 201, "")
}

func TestRegisterLoginLogout(t *testing.T) {
	e := newEnv(t)
	b := e.browser(t)
	const email, pw = "reader@example.com", "abc12345"

	// 未登录时查询登录状态返回 200，不产生 401
	r := b.get("/api/v1/auth/session")
	expect(t, r, 200, "")
	if _, ok := r.body["user"]; ok {
		t.Fatalf("未登录时不应返回 user：%v", r.body)
	}

	// 写请求必须带 CSRF 令牌
	r = b.do(http.MethodPost, "/api/v1/auth/email-codes", map[string]any{"email": email, "purpose": "register"}, false, nil)
	expect(t, r, 403, "CSRF_INVALID")

	expect(t, b.post("/api/v1/auth/email-codes", map[string]any{"email": email, "purpose": "register"}), 204, "")
	code := e.mailer.lastCode(t, email)
	expect(t, b.post("/api/v1/auth/email-codes", map[string]any{"email": email, "purpose": "register"}), 429, "TOO_MANY_REQUESTS")

	// 密码太弱时不消耗验证码
	expect(t, b.post("/api/v1/auth/register", map[string]any{"email": email, "code": code, "password": "short"}), 400, "WEAK_PASSWORD")
	expect(t, b.post("/api/v1/auth/register", map[string]any{"email": email, "code": "000000", "password": pw}), 400, "INVALID_CODE")
	r = b.post("/api/v1/auth/register", map[string]any{"email": email, "code": code, "password": pw})
	expect(t, r, 201, "")
	if b.cookie("xzji_session") == "" {
		t.Fatal("注册后应写入会话 Cookie")
	}
	if _, ok := r.body["token"]; ok {
		t.Fatal("网页注册不应在响应体返回令牌")
	}

	if r = b.get("/api/v1/auth/session"); r.body["user"] == nil {
		t.Fatalf("登录后应返回 user：%v", r.body)
	}
	r = b.get("/api/v1/me")
	expect(t, r, 200, "")
	if r.body["email"] != email || r.body["nickname"] != "reader" || r.body["emailVerified"] != true {
		t.Fatalf("用户信息不符：%v", r.body)
	}

	// 已注册邮箱不能再次发送注册验证码
	expect(t, b.post("/api/v1/auth/email-codes", map[string]any{"email": email, "purpose": "register"}), 409, "EMAIL_TAKEN")

	// 前台会话不能用于后台，即使把令牌放进后台 Cookie
	expect(t, b.get("/admin-api/v1/me"), 401, "UNAUTHORIZED")
	b.setCookie("xzji_admin_session", b.cookie("xzji_session"))
	expect(t, b.get("/admin-api/v1/me"), 401, "UNAUTHORIZED")

	expect(t, b.post("/api/v1/me/password", map[string]any{"oldPassword": "wrong1234", "newPassword": "new12345"}), 400, "WRONG_PASSWORD")
	expect(t, b.do(http.MethodPatch, "/api/v1/me", map[string]any{"nickname": "林间小路"}, true, nil), 200, "")

	expect(t, b.post("/api/v1/auth/logout", nil), 204, "")
	expect(t, b.get("/api/v1/me"), 401, "UNAUTHORIZED")

	// 再次登录
	expect(t, b.post("/api/v1/auth/login", map[string]any{"email": "Reader@Example.com ", "password": pw}), 200, "")
	r = b.get("/api/v1/me")
	expect(t, r, 200, "")
	if r.body["nickname"] != "林间小路" {
		t.Fatalf("昵称应已修改：%v", r.body)
	}
}

func TestLoginRateLimit(t *testing.T) {
	e := newEnv(t)
	b := e.browser(t)
	e.register(t, b, "victim@example.com", "abc12345")

	for i := 0; i < 5; i++ {
		expect(t, b.post("/api/v1/auth/login", map[string]any{"email": "victim@example.com", "password": "wrong1234"}), 401, "INVALID_CREDENTIALS")
	}
	// 连续失败 5 次后，正确密码也暂时拒绝
	expect(t, b.post("/api/v1/auth/login", map[string]any{"email": "victim@example.com", "password": "abc12345"}), 429, "TOO_MANY_REQUESTS")
	// 不存在的邮箱返回同样的错误
	expect(t, b.post("/api/v1/auth/login", map[string]any{"email": "nobody@example.com", "password": "abc12345"}), 401, "INVALID_CREDENTIALS")
}

func TestAppBearerLogin(t *testing.T) {
	e := newEnv(t)
	e.register(t, e.browser(t), "app@example.com", "abc12345")

	// App 不带 Origin 和 Cookie，不需要 CSRF 令牌
	app := e.browser(t)
	app.origin = ""
	r := app.do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "app@example.com", "password": "abc12345", "client": "app"}, false, nil)
	expect(t, r, 200, "")
	token, _ := r.body["token"].(string)
	if token == "" || app.cookie("xzji_session") != "" {
		t.Fatalf("App 登录应只在响应体返回令牌：%v", r.body)
	}
	expect(t, app.do(http.MethodGet, "/api/v1/me", nil, false, map[string]string{"Authorization": "Bearer " + token}), 200, "")
	expect(t, app.do(http.MethodGet, "/api/v1/me", nil, false, map[string]string{"Authorization": "Bearer bad"}), 401, "UNAUTHORIZED")
}

func TestPasswordReset(t *testing.T) {
	e := newEnv(t)
	b := e.browser(t)
	const email = "forget@example.com"
	e.register(t, b, email, "abc12345")

	// 未注册的邮箱也返回成功，但不发邮件
	other := e.browser(t)
	expect(t, other.post("/api/v1/auth/email-codes", map[string]any{"email": "ghost@example.com", "purpose": "reset_password"}), 204, "")
	if e.mailer.count("ghost@example.com") != 0 {
		t.Fatal("未注册邮箱不应收到邮件")
	}

	expect(t, other.post("/api/v1/auth/email-codes", map[string]any{"email": email, "purpose": "reset_password"}), 204, "")
	code := e.mailer.lastCode(t, email)
	expect(t, other.post("/api/v1/auth/password-reset", map[string]any{"email": email, "code": code, "password": "new12345"}), 204, "")
	// 验证码只能用一次
	expect(t, other.post("/api/v1/auth/password-reset", map[string]any{"email": email, "code": code, "password": "new12345"}), 400, "INVALID_CODE")

	// 重设后旧会话失效，新密码可以登录
	expect(t, b.get("/api/v1/me"), 401, "UNAUTHORIZED")
	expect(t, other.post("/api/v1/auth/login", map[string]any{"email": email, "password": "abc12345"}), 401, "INVALID_CREDENTIALS")
	expect(t, other.post("/api/v1/auth/login", map[string]any{"email": email, "password": "new12345"}), 200, "")
}

func TestAdminSessionAndUserManagement(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	admin, err := e.svc.Users.Create(ctx, user.CreateInput{Email: "boss@example.com", Password: "boss12345", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}

	member := e.browser(t)
	e.register(t, member, "member@example.com", "abc12345")

	// 后台同源访问：Origin 与 Host 相同
	a := e.browser(t)
	a.origin = e.srv.URL

	// 普通用户不能登录后台，提示与密码错误相同
	expect(t, a.post("/admin-api/v1/auth/login", map[string]any{"email": "member@example.com", "password": "abc12345"}), 401, "INVALID_CREDENTIALS")
	expect(t, a.post("/admin-api/v1/auth/login", map[string]any{"email": "boss@example.com", "password": "boss12345"}), 200, "")
	expect(t, a.get("/admin-api/v1/me"), 200, "")

	// 后台会话不能用于前台
	expect(t, a.get("/api/v1/me"), 401, "UNAUTHORIZED")
	a.setCookie("xzji_session", a.cookie("xzji_admin_session"))
	expect(t, a.get("/api/v1/me"), 401, "UNAUTHORIZED")

	r := a.get("/admin-api/v1/users?keyword=member")
	expect(t, r, 200, "")
	items, _ := r.body["items"].([]any)
	if len(items) != 1 || r.body["total"] != float64(1) {
		t.Fatalf("搜索结果不符：%v", r.body)
	}
	memberID := int64(items[0].(map[string]any)["id"].(float64))

	// 跨站来源的后台写请求被拒绝
	evil := *a
	evil.origin = "http://evil.test"
	expect(t, evil.do(http.MethodPatch, "/admin-api/v1/users/1", map[string]any{"status": "banned"}, true, nil), 403, "CSRF_ORIGIN")

	expect(t, a.do(http.MethodPatch, "/admin-api/v1/users/"+itoa(admin.ID), map[string]any{"status": "banned"}, true, nil), 400, "CANNOT_MODIFY_SELF")

	// 封禁后，该用户已登录的会话立即失效，也不能再登录
	expect(t, member.get("/api/v1/me"), 200, "")
	r = a.do(http.MethodPatch, "/admin-api/v1/users/"+itoa(memberID), map[string]any{"status": "banned"}, true, nil)
	expect(t, r, 200, "")
	if r.body["status"] != "banned" {
		t.Fatalf("状态应为 banned：%v", r.body)
	}
	expect(t, member.get("/api/v1/me"), 401, "UNAUTHORIZED")
	expect(t, member.post("/api/v1/auth/login", map[string]any{"email": "member@example.com", "password": "abc12345"}), 403, "USER_BANNED")

	expect(t, a.post("/admin-api/v1/auth/logout", nil), 204, "")
	expect(t, a.get("/admin-api/v1/users"), 401, "UNAUTHORIZED")
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
