// Package admin 实现后台接口（api/admin.yaml）。
package admin

import (
	"context"
	"database/sql"
	"log/slog"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"

	adminapi "xzji/internal/apigen/admin"
	"xzji/internal/buildinfo"
	"xzji/internal/httpapi/authn"
	"xzji/internal/module/auth"
	"xzji/internal/module/content"
	"xzji/internal/module/media"
	"xzji/internal/module/setting"
	"xzji/internal/module/user"
	platformdb "xzji/internal/platform/db"
)

type Deps struct {
	DB       *sql.DB
	Log      *slog.Logger
	Settings *setting.Service
	Auth     *auth.Service
	Users    *user.Service
	Realm    *authn.Realm
	CSRF     *authn.CSRF
	Media    *media.Service
	Content  *content.Service
	Home     *setting.HomeService
}

type Handler struct {
	Deps
}

var _ adminapi.StrictServerInterface = (*Handler)(nil)

func NewHandler(d Deps) *Handler { return &Handler{Deps: d} }

func ginCtx(ctx context.Context) *gin.Context { return ctx.(*gin.Context) }

// GetSystemStatus 数据库不可用时仍返回 200，由状态字段说明问题，方便后台展示。
func (h *Handler) GetSystemStatus(ctx context.Context, _ adminapi.GetSystemStatusRequestObject) (adminapi.GetSystemStatusResponseObject, error) {
	resp := adminapi.SystemStatus{
		Version:       buildinfo.Version,
		GoVersion:     runtime.Version(),
		UptimeSeconds: int64(time.Since(buildinfo.StartedAt).Seconds()),
		Database:      adminapi.DatabaseStatus{Status: adminapi.Up},
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	ver, err := platformdb.ServerVersion(ctx, h.DB)
	if err != nil {
		h.Log.Warn("系统状态：数据库不可用", "error", err)
		resp.Database.Status = adminapi.Down
		return adminapi.GetSystemStatus200JSONResponse(resp), nil
	}
	resp.Database.Version = ver

	if resp.Database.Migration, err = platformdb.MigrationVersion(ctx, h.DB); err != nil {
		return nil, err
	}
	if resp.SiteName, err = h.Settings.SiteName(ctx); err != nil {
		return nil, err
	}
	return adminapi.GetSystemStatus200JSONResponse(resp), nil
}
