// Package admin 实现后台接口（api/admin.yaml）。
package admin

import (
	"context"
	"database/sql"
	"log/slog"
	"runtime"
	"time"

	adminapi "xzji/internal/apigen/admin"
	"xzji/internal/buildinfo"
	"xzji/internal/module/setting"
	platformdb "xzji/internal/platform/db"
)

type Handler struct {
	db       *sql.DB
	log      *slog.Logger
	settings *setting.Service
}

var _ adminapi.StrictServerInterface = (*Handler)(nil)

func NewHandler(db *sql.DB, log *slog.Logger, settings *setting.Service) *Handler {
	return &Handler{db: db, log: log, settings: settings}
}

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

	ver, err := platformdb.ServerVersion(ctx, h.db)
	if err != nil {
		h.log.Warn("系统状态：数据库不可用", "error", err)
		resp.Database.Status = adminapi.Down
		return adminapi.GetSystemStatus200JSONResponse(resp), nil
	}
	resp.Database.Version = ver

	if resp.Database.Migration, err = platformdb.MigrationVersion(ctx, h.db); err != nil {
		return nil, err
	}
	if resp.SiteName, err = h.settings.SiteName(ctx); err != nil {
		return nil, err
	}
	return adminapi.GetSystemStatus200JSONResponse(resp), nil
}
