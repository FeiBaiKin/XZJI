package app

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"xzji/internal/config"
	"xzji/internal/module/frontsync"
)

// 公开接口只缓存匿名请求：带会话 Cookie 或 Bearer 的请求每次都重新取数据。
func TestPublicAPICache(t *testing.T) {
	e := newEnv(t)
	a := e.adminBrowser(t)
	cat := testCategory(t, a, "pc")
	publishedPost(t, a, cat, "缓存测试")

	visitor := e.browser(t)
	if r := visitor.get("/api/v1/posts"); r.header.Get("X-Cache") != "" {
		t.Fatalf("首次请求不该命中缓存，X-Cache = %q", r.header.Get("X-Cache"))
	}
	if r := visitor.get("/api/v1/posts"); r.header.Get("X-Cache") != "HIT" {
		t.Fatalf("匿名的重复请求应命中缓存，X-Cache = %q", r.header.Get("X-Cache"))
	}

	// 会话 Cookie 的请求不缓存，避免把个人数据发给别人
	logged := e.browser(t)
	e.register(t, logged, "cached@example.com", "abc12345")
	logged.get("/api/v1/posts")
	if r := logged.get("/api/v1/posts"); r.header.Get("X-Cache") != "" {
		t.Errorf("带会话 Cookie 的请求不该命中缓存，X-Cache = %q", r.header.Get("X-Cache"))
	}

	// Bearer 令牌的请求同样不缓存
	app := e.browser(t)
	expect(t, app.post("/api/v1/auth/email-codes", map[string]any{"email": "app@example.com", "purpose": "register"}), 204, "")
	r := app.post("/api/v1/auth/register", map[string]any{
		"email": "app@example.com", "code": e.mailer.lastCode(t, "app@example.com"), "password": "abc12345", "client": "app",
	})
	expect(t, r, 201, "")
	bearer := map[string]string{"Authorization": "Bearer " + r.body["token"].(string)}
	app.do(http.MethodGet, "/api/v1/posts", nil, false, bearer)
	if got := app.do(http.MethodGet, "/api/v1/posts", nil, false, bearer); got.header.Get("X-Cache") != "" {
		t.Errorf("带 Bearer 的请求不该命中缓存，X-Cache = %q", got.header.Get("X-Cache"))
	}

	// 个人数据接口本来就不在白名单里
	if r := visitor.get("/api/v1/me"); r.header.Get("X-Cache") != "" {
		t.Errorf("/me 不该命中缓存，X-Cache = %q", r.header.Get("X-Cache"))
	}
}

// 后台写请求成功后：后端缓存立刻清空，并通知前台服务器清空页面缓存。
func TestAdminWritePurgesCache(t *testing.T) {
	notified := make(chan http.Header, 8)
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		notified <- r.Header.Clone()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer front.Close()

	e := newEnv(t, func(c *config.Config) {
		c.Frontend = config.Frontend{PurgeURLs: []string{front.URL}, PurgeSecret: "test-secret"}
	})
	a := e.adminBrowser(t)
	cat := testCategory(t, a, "pc")
	publishedPost(t, a, cat, "第一篇")

	visitor := e.browser(t)
	visitor.get("/api/v1/posts")
	if r := visitor.get("/api/v1/posts"); r.header.Get("X-Cache") != "HIT" {
		t.Fatal("重复的匿名请求应命中缓存")
	}

	// 后台写入后，前台接口立刻能看到新内容
	newID := publishedPost(t, a, cat, "第二篇")
	r := visitor.get("/api/v1/posts")
	if r.header.Get("X-Cache") == "HIT" {
		t.Fatal("后台写入后不该再命中旧缓存")
	}
	if !containsPost(r, newID) {
		t.Error("清空缓存后应能看到刚发布的内容")
	}

	// 1 秒内的多次写入合并成一次通知，最多等 3 秒
	select {
	case h := <-notified:
		ts, err := strconv.ParseInt(h.Get(frontsync.HeaderTimestamp), 10, 64)
		if err != nil {
			t.Fatalf("清除通知的时间戳不是整数：%q", h.Get(frontsync.HeaderTimestamp))
		}
		if want := frontsync.Signature("test-secret", ts); want != h.Get(frontsync.HeaderSignature) {
			t.Errorf("清除通知的签名不对：%q", h.Get(frontsync.HeaderSignature))
		}
	case <-time.After(3 * time.Second):
		t.Fatal("后台写入后应通知前台清空缓存")
	}
}

func containsPost(r resp, id int64) bool {
	for _, it := range r.body["items"].([]any) {
		if num(it.(map[string]any)["id"]) == id {
			return true
		}
	}
	return false
}
