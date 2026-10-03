// Package db 嵌入数据库迁移文件，随程序发布。
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
