// xzji 后端程序入口：xzji serve | migrate | version。
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"

	"golang.org/x/term"

	"xzji/internal/app"
	"xzji/internal/buildinfo"
	"xzji/internal/config"
	"xzji/internal/module/user"
	platformdb "xzji/internal/platform/db"
	"xzji/internal/platform/mail"
)

const usage = `用法：
  xzji serve   [-c config.yaml]                 启动服务（默认自动执行数据库迁移）
  xzji migrate [-c config.yaml] up|down|status   执行、回滚或查看数据库迁移
  xzji migrate create <名称> [-dir db/migrations] 新建迁移文件
  xzji admin create -email <邮箱> [-nickname 昵称] [-promote]
                                                创建管理员；邮箱已注册时加 -promote 把它设为管理员
  xzji admin passwd -email <邮箱>                重设账号密码，该账号所有会话失效
  xzji media rebuild-variants [-c config.yaml] [-force]
                                                给已有图片补生成缩略图（-force 连已有的重新生成）
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
	case "admin":
		err = runAdmin(ctx, args)
	case "media":
		err = runMedia(ctx, args)
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

func runAdmin(ctx context.Context, args []string) error {
	if len(args) == 0 || (args[0] != "create" && args[0] != "passwd") {
		return errors.New("用法：xzji admin create|passwd -email <邮箱>")
	}
	sub := args[0]
	fs := flag.NewFlagSet("admin "+sub, flag.ExitOnError)
	cfgPath := fs.String("c", "config.yaml", "配置文件路径")
	emailArg := fs.String("email", "", "账号邮箱")
	nickname := fs.String("nickname", "", "昵称（create 时可选）")
	promote := fs.Bool("promote", false, "邮箱已注册时把该账号设为管理员")
	_ = fs.Parse(args[1:])

	email, err := user.NormalizeEmail(*emailArg)
	if err != nil {
		return errors.New("请用 -email 指定正确的邮箱")
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
	if err := platformdb.Migrate(ctx, db, log); err != nil {
		return err
	}
	mailer, err := mail.New(cfg.Mail, log)
	if err != nil {
		return err
	}
	svc, err := app.NewServices(db, mailer, log, cfg)
	if err != nil {
		return err
	}

	existing, err := svc.Users.GetByEmail(ctx, email)
	exists := err == nil
	if err != nil && !errors.Is(err, user.ErrNotFound) {
		return err
	}

	switch sub {
	case "create":
		if exists {
			if !*promote {
				return fmt.Errorf("%s 已经注册；如需把它设为管理员，请加上 -promote", email)
			}
			admin, active := user.RoleAdmin, user.StatusActive
			if _, err := svc.Users.AdminUpdate(ctx, user.Actor{}, existing.ID, user.AdminChange{Role: &admin, Status: &active}); err != nil {
				return err
			}
			fmt.Printf("已把 %s 设为管理员\n", email)
			return nil
		}
		password, err := readPassword(true)
		if err != nil {
			return err
		}
		u, err := svc.Users.Create(ctx, user.CreateInput{
			Email: email, Password: password, Nickname: *nickname, Role: user.RoleAdmin, EmailVerified: true,
		})
		if err != nil {
			return err
		}
		if err := svc.AdminLog.Record(ctx, 0, "admin.create", "user", u.ID, nil, "cli"); err != nil {
			return err
		}
		fmt.Printf("已创建管理员 %s（ID %d）\n", u.Email, u.ID)
	case "passwd":
		if !exists {
			return fmt.Errorf("%s 尚未注册", email)
		}
		password, err := readPassword(true)
		if err != nil {
			return err
		}
		if err := svc.Users.SetPassword(ctx, existing.ID, password, ""); err != nil {
			return err
		}
		if err := svc.AdminLog.Record(ctx, 0, "user.reset_password", "user", existing.ID, nil, "cli"); err != nil {
			return err
		}
		fmt.Printf("已重设 %s 的密码，该账号的所有登录会话已失效\n", email)
	}
	return nil
}

// readPassword 终端中隐藏输入并要求确认；非终端（管道）读取第一行，便于脚本调用。
func readPassword(confirm bool) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", errors.New("没有从标准输入读到密码")
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	fmt.Fprint(os.Stderr, "请输入密码（至少 8 位，同时包含字母和数字）：")
	first, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if confirm {
		fmt.Fprint(os.Stderr, "请再输入一次：")
		second, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		if string(first) != string(second) {
			return "", errors.New("两次输入的密码不一致")
		}
	}
	return string(first), nil
}
