// Package app 组装依赖并负责服务的启动与优雅退出。
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"xzji/internal/buildinfo"
	"xzji/internal/config"
	"xzji/internal/dbgen"
	"xzji/internal/httpapi"
	"xzji/internal/module/setting"
	platformdb "xzji/internal/platform/db"
)

// Serve 启动 HTTP 服务，ctx 取消后在 10 秒内处理完已有请求再退出。
func Serve(ctx context.Context, cfg *config.Config) error {
	log := NewLogger(cfg.Log)
	slog.SetDefault(log)

	db, err := platformdb.Open(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer db.Close()

	if cfg.DB.AutoMigrate {
		if err := platformdb.Migrate(ctx, db, log); err != nil {
			return err
		}
	}

	router, err := httpapi.NewRouter(httpapi.Deps{
		Config:   cfg,
		DB:       db,
		Log:      log,
		Settings: setting.NewService(dbgen.New(db)),
	})
	if err != nil {
		return fmt.Errorf("初始化路由：%w", err)
	}

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("服务已启动", "addr", cfg.HTTP.Addr, "env", cfg.Env, "version", buildinfo.Version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("HTTP 服务异常退出：%w", err)
	case <-ctx.Done():
	}

	log.Info("正在停止服务")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("停止服务：%w", err)
	}
	log.Info("服务已停止")
	return nil
}

func NewLogger(cfg config.Log) *slog.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.ToUpper(cfg.Level))); err != nil {
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	if cfg.Format == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
