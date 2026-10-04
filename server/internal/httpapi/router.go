// Package httpapi 装配 Gin 路由：前台接口、后台接口和后台页面。
package httpapi

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	adminapi "xzji/internal/apigen/admin"
	pubapi "xzji/internal/apigen/public"
	"xzji/internal/config"
	"xzji/internal/httpapi/admin"
	"xzji/internal/httpapi/adminui"
	"xzji/internal/httpapi/apierr"
	"xzji/internal/httpapi/authn"
	"xzji/internal/httpapi/middleware"
	"xzji/internal/httpapi/public"
	"xzji/internal/module/auth"
	"xzji/internal/module/comment"
	"xzji/internal/module/content"
	"xzji/internal/module/media"
	"xzji/internal/module/membership"
	"xzji/internal/module/session"
	"xzji/internal/module/setting"
	"xzji/internal/module/user"
)

const (
	PublicBase = "/api/v1"
	AdminBase  = "/admin-api/v1"

	CookiePublicSession = "xzji_session"
	CookiePublicCSRF    = "xzji_csrf"
	CookieAdminSession  = "xzji_admin_session"
	CookieAdminCSRF     = "xzji_admin_csrf"
)

type Deps struct {
	Config   *config.Config
	DB       *sql.DB
	Log      *slog.Logger
	Settings *setting.Service
	Auth     *auth.Service
	Users    *user.Service
	Sessions *session.Service
	Media    *media.Service
	Content  *content.Service
	Home     *setting.HomeService
	Members  *membership.Service
	Comments *comment.Service
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

	// 前台：Cookie 作用于主域，www 与 api 共享；同时接受 App 的 Bearer 令牌
	pubCookies := authn.Cookies{
		Session: CookiePublicSession, CSRF: CookiePublicCSRF,
		Domain: d.Config.Auth.CookieDomain, Secure: d.Config.Auth.CookieSecure, SameSite: http.SameSiteLaxMode,
	}
	pubRealm := &authn.Realm{
		Cookies: pubCookies, Bearer: true, Sessions: d.Sessions, Log: d.Log,
		Clients: map[session.Client]bool{session.ClientWeb: true, session.ClientApp: true},
	}
	pubCSRF := &authn.CSRF{Cookies: pubCookies, AllowOrigin: authn.OriginIn(d.Config.CORS.AllowOrigins)}

	// 后台：Cookie 只作用于后台域名并且 SameSite=Strict，只接受管理员的后台会话
	adminCookies := authn.Cookies{
		Session: CookieAdminSession, CSRF: CookieAdminCSRF,
		Secure: d.Config.Auth.CookieSecure, SameSite: http.SameSiteStrictMode,
	}
	adminRealm := &authn.Realm{
		Cookies: adminCookies, Role: user.RoleAdmin, Sessions: d.Sessions, Log: d.Log,
		Clients: map[session.Client]bool{session.ClientAdmin: true},
	}
	adminCSRF := &authn.CSRF{Cookies: adminCookies, AllowOrigin: authn.SameHost}

	pubHandler := public.NewHandler(public.Deps{
		DB: d.DB, Log: d.Log, Auth: d.Auth, Users: d.Users, Realm: pubRealm, CSRF: pubCSRF,
		Content: d.Content, Home: d.Home, Members: d.Members, Comments: d.Comments,
	})
	pubapi.RegisterHandlersWithOptions(r,
		pubapi.NewStrictHandlerWithOptions(pubHandler, []pubapi.StrictMiddlewareFunc{requirePublic}, pubapi.StrictGinServerOptions{
			RequestErrorHandlerFunc:  apierr.WriteBadRequest,
			HandlerErrorFunc:         apierr.Write,
			ResponseErrorHandlerFunc: apierr.Write,
		}),
		pubapi.GinServerOptions{
			BaseURL:      PublicBase,
			ErrorHandler: bindError,
			Middlewares:  []pubapi.MiddlewareFunc{pubCSRF.Check, pubRealm.Load},
		},
	)

	adminHandler := admin.NewHandler(admin.Deps{
		DB: d.DB, Log: d.Log, Settings: d.Settings, Auth: d.Auth, Users: d.Users, Realm: adminRealm, CSRF: adminCSRF,
		Media: d.Media, Content: d.Content, Home: d.Home, Members: d.Members, Comments: d.Comments,
	})
	adminapi.RegisterHandlersWithOptions(r,
		adminapi.NewStrictHandlerWithOptions(adminHandler, []adminapi.StrictMiddlewareFunc{requireAdmin}, adminapi.StrictGinServerOptions{
			RequestErrorHandlerFunc:  apierr.WriteBadRequest,
			HandlerErrorFunc:         apierr.Write,
			ResponseErrorHandlerFunc: apierr.Write,
		}),
		adminapi.GinServerOptions{
			BaseURL:      AdminBase,
			ErrorHandler: bindError,
			Middlewares:  []adminapi.MiddlewareFunc{adminCSRF.Check, adminRealm.Load},
		},
	)

	adminui.Mount(r)
	if d.Config.Media.Serve {
		uploads := r.Group("/uploads", func(c *gin.Context) {
			c.Header("X-Content-Type-Options", "nosniff")
			c.Header("Cache-Control", "public, max-age=2592000, immutable")
		})
		uploads.StaticFS("/", gin.Dir(d.Config.Media.Dir, false))
	}
	return r, nil
}

// requirePublic 契约中声明了 security 的前台接口必须已登录。
func requirePublic(f pubapi.StrictHandlerFunc, op string) pubapi.StrictHandlerFunc {
	if !pubapi.SecuredOperations[op] {
		return f
	}
	return func(c *gin.Context, req any) (any, error) {
		if _, ok := authn.FromContext(c); !ok {
			return nil, apierr.Unauthorized
		}
		return f(c, req)
	}
}

// requireAdmin 契约中声明了 security 的后台接口必须已登录。
func requireAdmin(f adminapi.StrictHandlerFunc, op string) adminapi.StrictHandlerFunc {
	if !adminapi.SecuredOperations[op] {
		return f
	}
	return func(c *gin.Context, req any) (any, error) {
		if _, ok := authn.FromContext(c); !ok {
			return nil, apierr.Unauthorized
		}
		return f(c, req)
	}
}

// bindError 处理路径和查询参数解析失败。
func bindError(c *gin.Context, err error, _ int) {
	apierr.WriteBadRequest(c, err)
}
