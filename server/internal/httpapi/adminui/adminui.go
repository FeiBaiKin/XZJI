// Package adminui 托管嵌入程序的后台单页应用。构建时由 Taskfile 把 admin/dist 复制到 dist 目录。
package adminui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var embedded embed.FS

const Prefix = "/admin"

const notBuiltHTML = `<!doctype html><meta charset="utf-8"><title>后台尚未构建</title>
<p style="font:14px/1.8 system-ui;padding:40px">后台页面还没有构建进程序。开发时运行 <code>task dev:admin</code> 并访问 Vite 开发服务器；发布前运行 <code>task build</code>。</p>`

// Mount 注册后台页面路由：存在的静态文件直接返回，其余路径交给前端路由，统一返回 index.html。
func Mount(r *gin.Engine) {
	dist, _ := fs.Sub(embedded, "dist")
	index, _ := fs.ReadFile(dist, "index.html")
	files := http.StripPrefix(Prefix, http.FileServer(http.FS(dist)))

	r.GET(Prefix, func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, Prefix+"/") })
	r.GET(Prefix+"/*path", func(c *gin.Context) {
		p := strings.TrimPrefix(c.Param("path"), "/")
		if p != "" && p != "index.html" {
			if st, err := fs.Stat(dist, p); err == nil && !st.IsDir() {
				// Vite 产物文件名带内容哈希，可以长期缓存
				if strings.HasPrefix(p, "assets/") {
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(c.Writer, c.Request)
				return
			}
		}
		c.Header("Cache-Control", "no-cache")
		if index == nil {
			c.Data(http.StatusServiceUnavailable, "text/html; charset=utf-8", []byte(notBuiltHTML))
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	})
}
