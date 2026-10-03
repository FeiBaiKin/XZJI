// Package public 实现前台接口（api/public.yaml）。
package public

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	pubapi "xzji/internal/apigen/public"
	"xzji/internal/buildinfo"
)

type Handler struct {
	db  *sql.DB
	log *slog.Logger
}

var _ pubapi.StrictServerInterface = (*Handler)(nil)

func NewHandler(db *sql.DB, log *slog.Logger) *Handler {
	return &Handler{db: db, log: log}
}

func (h *Handler) GetHealth(ctx context.Context, _ pubapi.GetHealthRequestObject) (pubapi.GetHealthResponseObject, error) {
	body := pubapi.Health{Status: pubapi.Ok, Version: buildinfo.Version, Database: pubapi.Up, Time: time.Now().UTC()}

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := h.db.PingContext(pingCtx); err != nil {
		h.log.Warn("健康检查：数据库不可用", "error", err)
		body.Status, body.Database = pubapi.Degraded, pubapi.Down
		return pubapi.GetHealth503JSONResponse(body), nil
	}
	return pubapi.GetHealth200JSONResponse(body), nil
}
