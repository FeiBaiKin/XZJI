package content

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"xzji/internal/dbgen"
	"xzji/internal/platform/ratelimit"
)

const (
	// viewWindow 同一 IP 对同一内容只计一次的窗口
	viewWindow = 30 * time.Minute
	// viewIPLimit 同一 IP 每分钟最多上报的次数。正常浏览远用不到，被刷时超出的请求仍返回 204，
	// 只是不计数；先判 IP 也避免给每个随机 ID 都建一个去重键。
	viewIPLimit  = 60
	viewIPWindow = time.Minute
	// viewPendingMax 内存里待写库的内容条数上限，满了之后丢弃新的 ID
	viewPendingMax = 2000
	// viewFlushBatch 单次刷库最多写多少条，避免被刷后一次打出成千上万条 UPDATE
	viewFlushBatch = 500
	// viewWarnInterval 丢弃上报时告警日志的最小间隔，被刷时不刷屏
	viewWarnInterval = time.Minute
)

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
	log     *slog.Logger
	// lastWarn 上次记录“丢弃上报”告警的时间；dropped 是那之后丢弃的条数
	lastWarn time.Time
	dropped  int64
}

func newViewCounter(w viewWriter, l ratelimit.Limiter, log *slog.Logger) *viewCounter {
	if log == nil {
		log = slog.Default()
	}
	return &viewCounter{pending: make(map[int64]int64), limiter: l, writer: w, window: viewWindow, log: log}
}

// Record 记录一次上报，同一 IP 在窗口内只计一次。超出 IP 限流或内存上限时丢弃。
func (v *viewCounter) Record(postID int64, ip string) {
	if postID <= 0 {
		return
	}
	if n, _ := v.limiter.Incr("view-ip:"+ip, viewIPWindow); n > viewIPLimit {
		return
	}
	if n, _ := v.limiter.Incr(fmt.Sprintf("view:%d:%s", postID, ip), v.window); n != 1 {
		return
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if _, ok := v.pending[postID]; !ok && len(v.pending) >= viewPendingMax {
		v.dropped++
		if v.lastWarn.IsZero() || time.Since(v.lastWarn) >= viewWarnInterval {
			v.log.Warn("浏览量上报已被丢弃", "原因", "待写库的内容条数达到上限", "上限", viewPendingMax,
				"丢弃条数", v.dropped, "内容ID", postID)
			v.lastWarn = time.Now()
			v.dropped = 0
		}
		return
	}
	v.pending[postID]++
}

// Flush 把内存里的计数写进数据库，一次最多写 viewFlushBatch 条，剩下的留到下次；
// 写失败的放回，等下次重试。
func (v *viewCounter) Flush(ctx context.Context) error {
	v.mu.Lock()
	pending := make(map[int64]int64, min(len(v.pending), viewFlushBatch))
	for id, n := range v.pending {
		if len(pending) >= viewFlushBatch {
			break
		}
		delete(v.pending, id)
		pending[id] = n
	}
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
