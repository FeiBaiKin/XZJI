package httpapi

import (
	"bytes"
	"net/http"

	"github.com/gin-gonic/gin"

	"xzji/internal/platform/cache"
)

// maxCacheBody 超过这个大小的响应不缓存，避免个别大响应把内存吃光
const maxCacheBody = 1 << 20

// cachedRoutes 可以缓存的公开接口，用路由模式（与契约里的路径一一对应）。
// 其余接口一律不缓存：解锁状态和评论带个人身份，auth、me、orders、pay 是个人数据，写请求也不在此列。
var cachedRoutes = map[string]bool{
	"/api/v1/site/home":         true,
	"/api/v1/categories":        true,
	"/api/v1/posts":             true,
	"/api/v1/posts/:id":         true,
	"/api/v1/posts/:id/related": true,
	"/api/v1/tags":              true,
	"/api/v1/tags/:slug":        true,
	"/api/v1/platforms":         true,
	"/api/v1/membership/levels": true,
	"/api/v1/membership/plans":  true,
	// sitemap 数据量大且随内容更新，同样缓存
	"/api/v1/seo/sitemap": true,
}

// cacheResponses 缓存匿名请求的公开接口响应。命中时直接写回缓存的响应体和 Content-Type，
// 并带上 X-Cache: HIT 便于排查；未命中的请求正常执行后再写入缓存。
func cacheResponses(store cache.Cache, sessionCookie string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cacheable(c, sessionCookie) {
			c.Next()
			return
		}
		key := c.Request.URL.RequestURI()
		if e, ok := store.Get(key); ok {
			c.Header("X-Cache", "HIT")
			c.Data(e.Status, e.ContentType, e.Body)
			c.Abort()
			return
		}
		w := &captureWriter{ResponseWriter: c.Writer}
		c.Writer = w
		c.Next()
		if w.Status() == http.StatusOK && !w.tooLarge {
			store.Set(key, cache.Entry{Status: http.StatusOK, ContentType: w.Header().Get("Content-Type"), Body: w.body.Bytes()})
		}
	}
}

// cacheable 只缓存匿名 GET 的白名单接口。带 Cookie 或 Bearer 的请求可能返回个人数据，
// 从根上排除，避免把某个人看到的内容发给别人。
func cacheable(c *gin.Context, sessionCookie string) bool {
	if c.Request.Method != http.MethodGet || !cachedRoutes[c.FullPath()] {
		return false
	}
	if c.GetHeader("Authorization") != "" {
		return false
	}
	_, err := c.Cookie(sessionCookie)
	return err != nil
}

// captureWriter 把响应体抄一份用于写入缓存，超过上限就不再抄，直接标记为不缓存。
type captureWriter struct {
	gin.ResponseWriter
	body     bytes.Buffer
	tooLarge bool
}

func (w *captureWriter) Write(b []byte) (int, error) {
	switch room := maxCacheBody - w.body.Len(); {
	case room <= 0:
		w.tooLarge = true
	case len(b) <= room:
		w.body.Write(b)
	default:
		w.body.Write(b[:room])
		w.tooLarge = true
	}
	return w.ResponseWriter.Write(b)
}

// 必须覆盖 WriteString：gin 的实现会直接调用底层 Write，绕过上面的拷贝。
func (w *captureWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }
