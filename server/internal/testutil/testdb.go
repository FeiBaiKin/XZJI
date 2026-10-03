// Package testutil 提供数据库集成测试的辅助函数。
package testutil

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"xzji/internal/config"
	platformdb "xzji/internal/platform/db"
)

// DB 连接 XZJI_TEST_DB_DSN 指向的测试库，清空所有表后重新执行迁移。
// 为防止误删数据，库名必须以 _test 结尾。未设置环境变量时跳过测试。
// 多个包同时使用会互相清库，目前只在 internal/app 中使用。
func DB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("XZJI_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("未设置 XZJI_TEST_DB_DSN，跳过数据库集成测试")
	}
	ctx := context.Background()
	db, err := platformdb.Open(ctx, config.DB{DSN: dsn, MaxOpenConns: 5, MaxIdleConns: 5, ConnMaxLifetime: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	var name string
	if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(name, "_test") {
		t.Fatalf("测试库名必须以 _test 结尾，当前是 %q，拒绝清空", name)
	}

	rows, err := db.QueryContext(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE()")
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var tbl string
		if err := rows.Scan(&tbl); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, "`"+tbl+"`")
	}
	rows.Close()
	if len(tables) > 0 {
		if _, err := db.ExecContext(ctx, "DROP TABLE "+strings.Join(tables, ", ")); err != nil {
			t.Fatal(err)
		}
	}
	if err := platformdb.Migrate(ctx, db, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	return db
}
