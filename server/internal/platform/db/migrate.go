package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/pressly/goose/v3"

	dbfs "xzji/db"
)

func newProvider(db *sql.DB) (*goose.Provider, error) {
	sub, err := fs.Sub(dbfs.Migrations, "migrations")
	if err != nil {
		return nil, err
	}
	return goose.NewProvider(goose.DialectMySQL, db, sub)
}

// Migrate 执行所有未应用的迁移。
func Migrate(ctx context.Context, db *sql.DB, log *slog.Logger) error {
	p, err := newProvider(db)
	if err != nil {
		return fmt.Errorf("初始化迁移：%w", err)
	}
	results, err := p.Up(ctx)
	for _, r := range results {
		log.Info("执行迁移", "version", r.Source.Version, "file", r.Source.Path, "duration", r.Duration)
	}
	if err != nil {
		return fmt.Errorf("执行迁移：%w", err)
	}
	return nil
}

// MigrateDown 回滚最近一次迁移。
func MigrateDown(ctx context.Context, db *sql.DB, log *slog.Logger) error {
	p, err := newProvider(db)
	if err != nil {
		return fmt.Errorf("初始化迁移：%w", err)
	}
	r, err := p.Down(ctx)
	if err != nil {
		return fmt.Errorf("回滚迁移：%w", err)
	}
	log.Info("回滚迁移", "version", r.Source.Version, "file", r.Source.Path)
	return nil
}

// MigrationStatus 列出每个迁移文件的应用状态。
func MigrationStatus(ctx context.Context, db *sql.DB) ([]*goose.MigrationStatus, error) {
	p, err := newProvider(db)
	if err != nil {
		return nil, fmt.Errorf("初始化迁移：%w", err)
	}
	return p.Status(ctx)
}

// MigrationVersion 返回数据库当前的迁移版本号。
func MigrationVersion(ctx context.Context, db *sql.DB) (int64, error) {
	p, err := newProvider(db)
	if err != nil {
		return 0, err
	}
	return p.GetDBVersion(ctx)
}

var migrationName = regexp.MustCompile(`^(\d+)_.+\.sql$`)
var safeName = regexp.MustCompile(`^[a-z0-9_]+$`)

// CreateMigration 在 dir 中按递增序号生成新的迁移文件，返回文件路径。
func CreateMigration(dir, name string) (string, error) {
	if !safeName.MatchString(name) {
		return "", fmt.Errorf("迁移名只能包含小写字母、数字和下划线：%q", name)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("读取迁移目录：%w", err)
	}
	var max int64
	for _, e := range entries {
		if m := migrationName.FindStringSubmatch(e.Name()); m != nil {
			if v, _ := strconv.ParseInt(m[1], 10, 64); v > max {
				max = v
			}
		}
	}
	path := filepath.Join(dir, fmt.Sprintf("%05d_%s.sql", max+1, name))
	tmpl := "-- +goose Up\n\n-- +goose Down\n"
	if err := os.WriteFile(path, []byte(tmpl), 0o644); err != nil {
		return "", fmt.Errorf("写入迁移文件：%w", err)
	}
	return path, nil
}
