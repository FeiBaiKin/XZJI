package content

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"xzji/internal/dbgen"
	"xzji/internal/platform/ratelimit"
)

// viewWindow 同一 IP 对同一内容只计一次的窗口
const viewWindow = 30 * time.Minute

// viewWriter 把累加的浏览量写进数据库。
type viewWriter interface {
	AddPostViews(ctx context.Context, arg dbgen.AddPostViewsParams) error
}

// viewCounter 先在内存里累加浏览量，由定时任务定期写库，避免每次访问都写数据库。
// 去重交给 ratelimit：键按 IP 和内容区分，窗口内第二次上报不再计数。
type viewCounter struct {
	mu      sync.Mutex
	pending map[int64]int64
	limiter ratelimit.Limiter
	writer  viewWriter
	window  time.Duration
}

func newViewCounter(w viewWriter, l ratelimit.Limiter) *viewCounter {
	return &viewCounter{pending: make(map[int64]int64), limiter: l, writer: w, window: viewWindow}
}

// Record 记录一次上报，同一 IP 在窗口内只计一次。
func (v *viewCounter) Record(postID int64, ip string) {
	if postID <= 0 {
		return
	}
	if n, _ := v.limiter.Incr(fmt.Sprintf("view:%d:%s", postID, ip), v.window); n != 1 {
		return
	}
	v.mu.Lock()
	v.pending[postID]++
	v.mu.Unlock()
}

// Flush 把内存里的计数写进数据库；写失败的放回，等下次重试。
func (v *viewCounter) Flush(ctx context.Context) error {
	v.mu.Lock()
	pending := v.pending
	v.pending = make(map[int64]int64)
	v.mu.Unlock()
	if len(pending) == 0 {
		return nil
	}

	var errs []error
	for id, n := range pending {
		if err := v.writer.AddPostViews(ctx, dbgen.AddPostViewsParams{ID: id, ViewCount: int32(min(n, int64(1<<31-1)))}); err != nil {
			v.mu.Lock()
			v.pending[id] += n
			v.mu.Unlock()
			errs = append(errs, fmt.Errorf("写入内容 %d 的浏览量：%w", id, err))
		}
	}
	return errors.Join(errs...)
}

// RecordView 上报一次浏览量，只累加在内存里。
func (s *Service) RecordView(postID int64, ip string) { s.views.Record(postID, ip) }

// FlushViews 把内存里累加的浏览量写进数据库。
func (s *Service) FlushViews(ctx context.Context) error { return s.views.Flush(ctx) }
