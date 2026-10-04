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
	"xzji/internal/module/access"
	"xzji/internal/module/adminlog"
	"xzji/internal/module/auth"
	"xzji/internal/module/comment"
	"xzji/internal/module/content"
	"xzji/internal/module/media"
	"xzji/internal/module/membership"
	"xzji/internal/module/order"
	"xzji/internal/module/payment"
	"xzji/internal/module/session"
	"xzji/internal/module/setting"
	"xzji/internal/module/user"
	"xzji/internal/module/verifycode"
	"xzji/internal/module/wallet"
	platformdb "xzji/internal/platform/db"
	"xzji/internal/platform/mail"
	"xzji/internal/platform/ratelimit"
	"xzji/internal/platform/scheduler"
	"xzji/internal/platform/secret"
	"xzji/internal/platform/storage"
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
	Media    *media.Service
	Content  *content.Service
	Home     *setting.HomeService
	Members  *membership.Service
	Comments *comment.Service
	Access   *access.Service
	Wallet   *wallet.Service
	Orders   *order.Service
	Payment  *payment.Service
}

func NewServices(db *sql.DB, mailer mail.Sender, log *slog.Logger, cfg *config.Config) (*Services, error) {
	mediaCfg := cfg.Media
	q := dbgen.New(db)
	s := &Services{
		Settings: setting.NewService(q),
		Sessions: session.NewService(q),
		AdminLog: adminlog.NewService(q),
		Limiter:  ratelimit.NewMemory(),
		Media:    media.NewService(q, storage.NewLocal(mediaCfg.Dir), mediaCfg.BaseURL, int64(mediaCfg.MaxSizeMB)<<20),
	}
	s.Home = setting.NewHomeService(s.Settings, s.Media)
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
	s.Members = membership.NewService(q, s.Users, s.AdminLog)
	s.Content = content.NewService(db, s.Media, s.Users, s.AdminLog, s.Limiter, cfg.Location())
	s.Comments = comment.NewService(q, s.Content, s.Users, s.Members, s.Settings, s.Limiter)
	s.Wallet = wallet.NewService(db, s.Users, s.AdminLog)
	s.Orders = order.NewService(db, s.Content, s.Members, s.Wallet, s.AdminLog, s.Limiter, cfg.Location())
	s.Access = access.NewService(s.Members, s.Comments, s.Orders)
	s.Content.SetViewerResolver(s.Access)
	s.Orders.SetViewerResolver(s.Access)
	box, err := secret.LoadOrCreate(cfg.SecretKeyFile)
	if err != nil {
		return nil, err
	}
	s.Payment = payment.NewService(s.Settings, box, s.Orders, s.AdminLog, s.Limiter, log, cfg.Pay.Mock)
	return s, nil
}

func NewRouter(cfg *config.Config, db *sql.DB, log *slog.Logger, s *Services) (*gin.Engine, error) {
	return httpapi.NewRouter(httpapi.Deps{
		Config: cfg, DB: db, Log: log,
		Settings: s.Settings, Auth: s.Auth, Users: s.Users, Sessions: s.Sessions,
		Media: s.Media, Content: s.Content, Home: s.Home, Members: s.Members, Comments: s.Comments,
		Orders: s.Orders, Payment: s.Payment, Wallet: s.Wallet,
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
	svc, err := NewServices(db, mailer, log, cfg)
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
	go scheduler.Every(ctx, log, "清理过期会员", time.Hour, svc.Members.CleanupExpired)
	go scheduler.Every(ctx, log, "支付补单", time.Minute, svc.Payment.Reconcile)
	go scheduler.Every(ctx, log, "关闭超时订单", time.Minute, svc.Payment.CloseExpired)
	if cfg.Pay.Mock {
		log.Warn("模拟支付已开启：微信、支付宝不连真实渠道，只能用于开发环境")
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
