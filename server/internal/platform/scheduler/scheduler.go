// Package scheduler 运行进程内的周期任务。
package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Every 每隔 interval 执行一次 fn，直到 ctx 取消；fn 出错只记录日志，不中断后续执行。
func Every(ctx context.Context, log *slog.Logger, name string, interval time.Duration, fn func(context.Context) error) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := fn(ctx); err != nil {
				log.Error("定时任务失败", "task", name, "error", err)
			}
		}
	}
}
