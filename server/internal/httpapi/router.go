// Package httpapi 装配 Gin 路由：前台接口、后台接口和后台页面。
package httpapi

import (
	"database/sql"
	"log/slog"

	"github.com/gin-gonic/gin"

	adminapi "xzji/internal/apigen/admin"
	pubapi "xzji/internal/apigen/public"
	"xzji/internal/config"
	"xzji/internal/httpapi/admin"
	"xzji/internal/httpapi/adminui"
	"xzji/internal/httpapi/apierr"
	"xzji/internal/httpapi/middleware"
	"xzji/internal/httpapi/public"
	"xzji/internal/module/setting"
)

const (
	PublicBase = "/api/v1"
	AdminBase  = "/admin-api/v1"
)

type Deps struct {
	Config   *config.Config
	DB       *sql.DB
	Log      *slog.Logger
	Settings *setting.Service
}

func NewRouter(d Deps) (*gin.Engine, error) {
	if d.Config.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	// 只信任自家 Nginx 传来的 X-Real-IP，客户端伪造的 X-Forwarded-For 一律忽略
	r.RemoteIPHeaders = []string{"X-Real-IP"}
	if err := r.SetTrustedProxies(d.Config.HTTP.TrustedProxies); err != nil {
		return nil, err
	}
	r.HandleMethodNotAllowed = true
	r.Use(
		middleware.RequestID(),
		middleware.AccessLog(d.Log),
		middleware.Recovery(d.Log),
		middleware.CORS(d.Config.CORS.AllowOrigins),
	)
	r.NoRoute(func(c *gin.Context) { apierr.Write(c, apierr.NotFound) })
	r.NoMethod(func(c *gin.Context) { apierr.Write(c, apierr.MethodNotAllowed) })

	pubapi.RegisterHandlersWithOptions(r,
		pubapi.NewStrictHandlerWithOptions(public.NewHandler(d.DB, d.Log), nil, pubapi.StrictGinServerOptions{
			RequestErrorHandlerFunc:  apierr.WriteBadRequest,
			HandlerErrorFunc:         apierr.Write,
			ResponseErrorHandlerFunc: apierr.Write,
		}),
		pubapi.GinServerOptions{BaseURL: PublicBase, ErrorHandler: bindError},
	)

	adminapi.RegisterHandlersWithOptions(r,
		adminapi.NewStrictHandlerWithOptions(admin.NewHandler(d.DB, d.Log, d.Settings), nil, adminapi.StrictGinServerOptions{
			RequestErrorHandlerFunc:  apierr.WriteBadRequest,
			HandlerErrorFunc:         apierr.Write,
			ResponseErrorHandlerFunc: apierr.Write,
		}),
		adminapi.GinServerOptions{BaseURL: AdminBase, ErrorHandler: bindError},
	)

	adminui.Mount(r)
	return r, nil
}

// bindError 处理路径和查询参数解析失败。
func bindError(c *gin.Context, err error, _ int) {
	apierr.WriteBadRequest(c, err)
}
