// xzji 后端程序入口：xzji serve | migrate | version。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"

	"xzji/internal/app"
	"xzji/internal/buildinfo"
	"xzji/internal/config"
	platformdb "xzji/internal/platform/db"
)

const usage = `用法：
  xzji serve   [-c config.yaml]                 启动服务（默认自动执行数据库迁移）
  xzji migrate [-c config.yaml] up|down|status   执行、回滚或查看数据库迁移
  xzji migrate create <名称> [-dir db/migrations] 新建迁移文件
  xzji version                                  查看版本
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch cmd, args := os.Args[1], os.Args[2:]; cmd {
	case "serve":
		err = runServe(ctx, args)
	case "migrate":
		err = runMigrate(ctx, args)
	case "version":
		fmt.Println(buildinfo.Version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "未知命令：%s\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		os.Exit(1)
	}
}

func runServe(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	cfgPath := fs.String("c", "config.yaml", "配置文件路径")
	_ = fs.Parse(args)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	return app.Serve(ctx, cfg)
}

func runMigrate(ctx context.Context, args []string) error {
	if len(args) > 0 && args[0] == "create" {
		fs := flag.NewFlagSet("migrate create", flag.ExitOnError)
		dir := fs.String("dir", "db/migrations", "迁移文件目录")
		// 名称写在参数前后都可以
		rest, name := args[1:], ""
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			name, rest = rest[0], rest[1:]
		}
		_ = fs.Parse(rest)
		if name == "" && fs.NArg() == 1 {
			name = fs.Arg(0)
		} else if fs.NArg() > 0 {
			name = ""
		}
		if name == "" {
			return errors.New("请提供一个迁移名称，例如：xzji migrate create add_users")
		}
		path, err := platformdb.CreateMigration(*dir, name)
		if err != nil {
			return err
		}
		fmt.Println("已创建", path)
		return nil
	}

	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	cfgPath := fs.String("c", "config.yaml", "配置文件路径")
	_ = fs.Parse(args)
	action := fs.Arg(0)
	if action == "" {
		action = "up"
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	log := app.NewLogger(cfg.Log)
	db, err := platformdb.Open(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer db.Close()

	switch action {
	case "up":
		return platformdb.Migrate(ctx, db, log)
	case "down":
		return platformdb.MigrateDown(ctx, db, log)
	case "status":
		list, err := platformdb.MigrationStatus(ctx, db)
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "版本\t状态\t应用时间\t文件")
		for _, s := range list {
			applied := "-"
			if !s.AppliedAt.IsZero() {
				applied = s.AppliedAt.Format("2006-01-02 15:04:05")
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", s.Source.Version, s.State, applied, s.Source.Path)
		}
		return w.Flush()
	default:
		return fmt.Errorf("未知的迁移操作：%s（可用 up、down、status、create）", action)
	}
}
