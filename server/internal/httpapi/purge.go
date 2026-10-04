package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Purger 清空缓存：先清后端缓存，再通知各前台服务器。由 frontsync.Service 实现，测试里可注入假实现。
type Purger interface {
	Purge()
}

// purgeOnAdminWrite 后台写请求成功后清空缓存。内容、分类、标签、首页设置、评论审核、
// 会员等级与套餐都由它覆盖，不用逐个 handler 加；登录和当前用户接口不涉及前台内容，跳过。
func purgeOnAdminWrite(p Purger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if p == nil || c.Writer.Status() >= http.StatusBadRequest || !isAdminWrite(c.Request.Method, c.Request.URL.Path) {
			return
		}
		p.Purge()
	}
}

func isAdminWrite(method, path string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return false
	}
	rest, ok := strings.CutPrefix(path, AdminBase)
	if !ok {
		return false
	}
	return rest != "/me" && !strings.HasPrefix(rest, "/me/") && !strings.HasPrefix(rest, "/auth/")
}
