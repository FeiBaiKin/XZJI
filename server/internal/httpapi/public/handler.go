// Package public 实现前台接口（api/public.yaml）。
package public

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	pubapi "xzji/internal/apigen/public"
	"xzji/internal/buildinfo"
	"xzji/internal/httpapi/authn"
	"xzji/internal/module/auth"
	"xzji/internal/module/user"
)

type Deps struct {
	DB    *sql.DB
	Log   *slog.Logger
	Auth  *auth.Service
	Users *user.Service
	Realm *authn.Realm
	CSRF  *authn.CSRF
}

type Handler struct {
	Deps
}

var _ pubapi.StrictServerInterface = (*Handler)(nil)

func NewHandler(d Deps) *Handler { return &Handler{Deps: d} }

func (h *Handler) GetHealth(ctx context.Context, _ pubapi.GetHealthRequestObject) (pubapi.GetHealthResponseObject, error) {
	body := pubapi.Health{Status: pubapi.Ok, Version: buildinfo.Version, Database: pubapi.Up, Time: time.Now().UTC()}

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := h.DB.PingContext(pingCtx); err != nil {
		h.Log.Warn("健康检查：数据库不可用", "error", err)
		body.Status, body.Database = pubapi.Degraded, pubapi.Down
		return pubapi.GetHealth503JSONResponse(body), nil
	}
	return pubapi.GetHealth200JSONResponse(body), nil
}

// ginCtx strict handler 收到的 ctx 就是 *gin.Context，需要读写 Cookie 时取回。
func ginCtx(ctx context.Context) *gin.Context { return ctx.(*gin.Context) }

func meta(c *gin.Context) auth.Meta {
	return auth.Meta{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()}
}

func toUser(u user.User) pubapi.User {
	return pubapi.User{
		Id:            u.ID,
		Email:         u.Email,
		Nickname:      u.Nickname,
		Role:          pubapi.UserRole(u.Role),
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt,
	}
}
