// Package buildinfo 保存构建版本与进程启动时间。
package buildinfo

import "time"

// Version 构建时通过 -ldflags "-X xzji/internal/buildinfo.Version=v0.1.0" 注入。
var Version = "dev"

var StartedAt = time.Now()
