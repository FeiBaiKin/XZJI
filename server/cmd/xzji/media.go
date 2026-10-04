package main

import (
	"context"
	"errors"
	"flag"
	"fmt"

	"xzji/internal/app"
	"xzji/internal/config"
	"xzji/internal/dbgen"
	"xzji/internal/module/media"
	platformdb "xzji/internal/platform/db"
	"xzji/internal/platform/storage"
)

// runMedia 处理 media 子命令，目前只有给已有图片补生成缩略图。
func runMedia(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "rebuild-variants" {
		return errors.New("用法：xzji media rebuild-variants [-c config.yaml] [-force]")
	}
	fs := flag.NewFlagSet("media rebuild-variants", flag.ExitOnError)
	cfgPath := fs.String("c", "config.yaml", "配置文件路径")
	force := fs.Bool("force", false, "已有的缩略图也重新生成")
	_ = fs.Parse(args[1:])

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

	svc := media.NewService(dbgen.New(db), storage.NewLocal(cfg.Media.Dir), cfg.Media.BaseURL, int64(cfg.Media.MaxSizeMB)<<20, log)
	done, failed, err := svc.RebuildVariants(ctx, *force)
	if err != nil {
		return err
	}
	fmt.Printf("已生成缩略图 %d 张，失败 %d 张\n", done, failed)
	if failed > 0 {
		return errors.New("有图片生成失败，详见日志")
	}
	return nil
}
