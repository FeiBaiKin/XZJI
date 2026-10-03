// Package db 负责数据库连接、事务和迁移。
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"

	"xzji/internal/config"
)

// Open 建立连接池。无论 DSN 怎么写，都强制使用 UTC 时区和 utf8mb4，保证时间与字符集一致。
func Open(ctx context.Context, cfg config.DB) (*sql.DB, error) {
	mc, err := mysql.ParseDSN(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("解析数据库 DSN：%w", err)
	}
	mc.ParseTime = true
	mc.Loc = time.UTC
	mc.Collation = "utf8mb4_unicode_ci"
	if mc.Params == nil {
		mc.Params = map[string]string{}
	}
	mc.Params["time_zone"] = "'+00:00'"

	connector, err := mysql.NewConnector(mc)
	if err != nil {
		return nil, fmt.Errorf("创建数据库连接器：%w", err)
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("连接数据库：%w", err)
	}
	return db, nil
}

// WithTx 在事务中执行 fn，fn 返回错误或发生 panic 时回滚。
func WithTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启事务：%w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交事务：%w", err)
	}
	return nil
}

// ServerVersion 返回数据库服务端版本。
func ServerVersion(ctx context.Context, db *sql.DB) (string, error) {
	var v string
	err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&v)
	return v, err
}
