package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"xzji/internal/platform/cache"
)

// 测试路由只注册缓存中间件关心的几条，路径与契约保持一致。
func newCacheTestRouter(store cache.Cache, h gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(cacheResponses(store, CookiePublicSession))
	for _, m := range []string{http.MethodGet, http.MethodPost} {
		r.Handle(m, "/api/v1/posts", h)
	}
	r.GET("/api/v1/posts/:id", h)
	r.GET("/api/v1/posts/:id/unlocked", h)
	r.GET("/api/v1/me", h)
	return r
}

// counter 返回一个自增计数器和对应的处理函数。
func counter(status int, body string) (*int, gin.HandlerFunc) {
	n := 0
	return &n, func(c *gin.Context) {
		n++
		c.String(status, body)
	}
}

func do(t *testing.T, r *gin.Engine, method, target string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCacheResponsesHitRules(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		header map[string]string
		// runs 第二次请求后处理函数的执行次数：1 表示没命中缓存
		runs int
	}{
		{name: "匿名 GET 命中缓存", method: http.MethodGet, target: "/api/v1/posts", runs: 1},
		{name: "带查询串也能命中", method: http.MethodGet, target: "/api/v1/posts?page=2", runs: 1},
		{name: "路径参数命中", method: http.MethodGet, target: "/api/v1/posts/7", runs: 1},
		{name: "带会话 Cookie 不缓存", method: http.MethodGet, target: "/api/v1/posts",
			header: map[string]string{"Cookie": CookiePublicSession + "=abc"}, runs: 2},
		{name: "带 Bearer 不缓存", method: http.MethodGet, target: "/api/v1/posts",
			header: map[string]string{"Authorization": "Bearer xyz"}, runs: 2},
		{name: "不在白名单的接口不缓存", method: http.MethodGet, target: "/api/v1/posts/7/unlocked", runs: 2},
		{name: "个人数据接口不缓存", method: http.MethodGet, target: "/api/v1/me", runs: 2},
		{name: "写请求不缓存", method: http.MethodPost, target: "/api/v1/posts", runs: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runs, h := counter(http.StatusOK, "body")
			r := newCacheTestRouter(cache.NewLRU(10, 0), h)
			do(t, r, tt.method, tt.target, tt.header)
			do(t, r, tt.method, tt.target, tt.header)
			if *runs != tt.runs {
				t.Errorf("处理函数执行了 %d 次，期望 %d 次", *runs, tt.runs)
			}
		})
	}
}

// 查询串不同要分开缓存，不能把第 1 页的结果给第 2 页。
func TestCacheResponsesKeyIncludesQuery(t *testing.T) {
	runs, h := counter(http.StatusOK, "body")
	r := newCacheTestRouter(cache.NewLRU(10, 0), h)
	do(t, r, http.MethodGet, "/api/v1/posts?page=1", nil)
	do(t, r, http.MethodGet, "/api/v1/posts?page=2", nil)
	if *runs != 2 {
		t.Errorf("两个查询串应各执行一次，实际 %d 次", *runs)
	}
	// 再次请求第一页时命中缓存
	do(t, r, http.MethodGet, "/api/v1/posts?page=1", nil)
	if *runs != 2 {
		t.Errorf("重复的查询串应命中缓存，实际执行 %d 次", *runs)
	}
}

func TestCacheResponsesOnlyCachesOK(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusInternalServerError, http.StatusForbidden} {
		runs, h := counter(status, "err")
		r := newCacheTestRouter(cache.NewLRU(10, 0), h)
		w := do(t, r, http.MethodGet, "/api/v1/posts", nil)
		do(t, r, http.MethodGet, "/api/v1/posts", nil)
		if *runs != 2 {
			t.Errorf("状态码 %d 不应被缓存，处理函数执行了 %d 次", status, *runs)
		}
		if w.Code != status {
			t.Errorf("响应状态码 = %d，期望 %d", w.Code, status)
		}
	}
}

func TestCacheResponsesReplaysBody(t *testing.T) {
	_, h := counter(http.StatusOK, `{"ok":true}`)
	r := newCacheTestRouter(cache.NewLRU(10, 0), h)
	first := do(t, r, http.MethodGet, "/api/v1/posts", nil)
	second := do(t, r, http.MethodGet, "/api/v1/posts", nil)

	if first.Body.String() != second.Body.String() {
		t.Errorf("命中缓存时的响应体应与首次一致，%q != %q", first.Body.String(), second.Body.String())
	}
	if first.Header().Get("X-Cache") != "" {
		t.Error("首次请求不应带 X-Cache 响应头")
	}
	if second.Header().Get("X-Cache") != "HIT" {
		t.Errorf("命中缓存的响应应带 X-Cache: HIT，实际 %q", second.Header().Get("X-Cache"))
	}
}

func TestCacheResponsesPurge(t *testing.T) {
	store := cache.NewLRU(10, 0)
	runs, h := counter(http.StatusOK, "body")
	r := newCacheTestRouter(store, h)
	do(t, r, http.MethodGet, "/api/v1/posts", nil)
	do(t, r, http.MethodGet, "/api/v1/posts", nil)
	if *runs != 1 {
		t.Fatalf("Purge 前应命中缓存，处理函数执行了 %d 次", *runs)
	}
	store.Purge()
	do(t, r, http.MethodGet, "/api/v1/posts", nil)
	if *runs != 2 {
		t.Errorf("Purge 后应重新执行处理函数，实际执行 %d 次", *runs)
	}
}
