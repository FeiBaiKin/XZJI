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
	"xzji/internal/module/comment"
	"xzji/internal/module/content"
	"xzji/internal/module/membership"
	"xzji/internal/module/setting"
	"xzji/internal/module/user"
)

type Deps struct {
	DB       *sql.DB
	Log      *slog.Logger
	Auth     *auth.Service
	Users    *user.Service
	Realm    *authn.Realm
	CSRF     *authn.CSRF
	Content  *content.Service
	Home     *setting.HomeService
	Members  *membership.Service
	Comments *comment.Service
}

type Handler struct {
	Deps
}

var _ pubapi.StrictServerInterface = (*Handler)(nil)

func NewHandler(d Deps) *Handler { return &Handler{Deps: d} }

func (h *Handler) GetHealth(ctx context.Context, _ pubapi.GetHealthRequestObject) (pubapi.GetHealthResponseObject, error) {
	body := pubapi.Health{Status: pubapi.HealthStatusOk, Version: buildinfo.Version, Database: pubapi.Up, Time: time.Now().UTC()}

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := h.DB.PingContext(pingCtx); err != nil {
		h.Log.Warn("健康检查：数据库不可用", "error", err)
		body.Status, body.Database = pubapi.HealthStatusDegraded, pubapi.Down
		return pubapi.GetHealth503JSONResponse(body), nil
	}
	return pubapi.GetHealth200JSONResponse(body), nil
}

// ginCtx strict handler 收到的 ctx 就是 *gin.Context，需要读写 Cookie 时取回。
func ginCtx(ctx context.Context) *gin.Context { return ctx.(*gin.Context) }

func meta(c *gin.Context) auth.Meta {
	return auth.Meta{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()}
}

// toUser 附带有效的会员状态。会员状态只用于展示，查询失败时记日志后省略，不影响接口本身。
func (h *Handler) toUser(ctx context.Context, u user.User) pubapi.User {
	out := pubapi.User{
		Id:            u.ID,
		Email:         u.Email,
		Nickname:      u.Nickname,
		Role:          pubapi.UserRole(u.Role),
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt,
	}
	m, err := h.Members.Get(ctx, u.ID)
	if err != nil {
		h.Log.Warn("查询会员状态失败", "user_id", u.ID, "error", err)
		return out
	}
	if m != nil && m.Active(time.Now()) {
		out.Membership = &pubapi.UserMembership{LevelId: m.Level.ID, Name: m.Level.Name, Rank: m.Level.Rank, ExpiresAt: m.ExpiresAt}
	}
	return out
}
