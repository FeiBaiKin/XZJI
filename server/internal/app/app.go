// Package app 组装依赖并负责服务的启动与优雅退出。
package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"xzji/internal/buildinfo"
	"xzji/internal/config"
	"xzji/internal/dbgen"
	"xzji/internal/httpapi"
	"xzji/internal/module/adminlog"
	"xzji/internal/module/auth"
	"xzji/internal/module/session"
	"xzji/internal/module/setting"
	"xzji/internal/module/user"
	"xzji/internal/module/verifycode"
	platformdb "xzji/internal/platform/db"
	"xzji/internal/platform/mail"
	"xzji/internal/platform/ratelimit"
	"xzji/internal/platform/scheduler"
)

// Services 所有业务服务，服务进程、命令行和测试共用同一套装配。
type Services struct {
	Settings *setting.Service
	Sessions *session.Service
	Users    *user.Service
	Codes    *verifycode.Service
	Auth     *auth.Service
	AdminLog *adminlog.Service
	Limiter  *ratelimit.Memory
}

func NewServices(db *sql.DB, mailer mail.Sender, log *slog.Logger) (*Services, error) {
	q := dbgen.New(db)
	s := &Services{
		Settings: setting.NewService(q),
		Sessions: session.NewService(q),
		AdminLog: adminlog.NewService(q),
		Limiter:  ratelimit.NewMemory(),
	}
	users, err := user.NewService(q, user.NewHasher(4), s.Sessions, s.AdminLog)
	if err != nil {
		return nil, err
	}
	s.Users = users
	siteName := func(ctx context.Context) string {
		name, err := s.Settings.SiteName(ctx)
		if err != nil {
			return "XZJI"
		}
		return name
	}
	s.Codes = verifycode.NewService(q, mailer, s.Limiter, siteName, log)
	s.Auth = auth.NewService(s.Users, s.Sessions, s.Codes, s.Limiter)
	return s, nil
}

func NewRouter(cfg *config.Config, db *sql.DB, log *slog.Logger, s *Services) (*gin.Engine, error) {
	return httpapi.NewRouter(httpapi.Deps{
		Config: cfg, DB: db, Log: log,
		Settings: s.Settings, Auth: s.Auth, Users: s.Users, Sessions: s.Sessions,
	})
}

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

	mailer, err := mail.New(cfg.Mail, log)
	if err != nil {
		return err
	}
	svc, err := NewServices(db, mailer, log)
	if err != nil {
		return err
	}
	router, err := NewRouter(cfg, db, log, svc)
	if err != nil {
		return fmt.Errorf("初始化路由：%w", err)
	}

	go svc.Limiter.Sweep(ctx, 5*time.Minute)
	go scheduler.Every(ctx, log, "清理过期会话", time.Hour, svc.Sessions.Cleanup)
	go scheduler.Every(ctx, log, "清理过期验证码", time.Hour, svc.Codes.Cleanup)

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
