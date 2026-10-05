package content

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"

	"xzji/internal/dbgen"
	"xzji/internal/platform/ratelimit"
)

// fakeViewWriter 累计写入的计数，fail 里的内容一直写失败。
type fakeViewWriter struct {
	mu    sync.Mutex
	calls map[int64]int32
	fail  map[int64]bool
}

func newFakeViewWriter(fail ...int64) *fakeViewWriter {
	f := &fakeViewWriter{calls: map[int64]int32{}, fail: map[int64]bool{}}
	for _, id := range fail {
		f.fail[id] = true
	}
	return f
}

func (f *fakeViewWriter) AddPostViews(_ context.Context, arg dbgen.AddPostViewsParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail[arg.ID] {
		return errors.New("写库失败")
	}
	f.calls[arg.ID] += arg.ViewCount
	return nil
}

func (f *fakeViewWriter) count(id int64) int32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[id]
}

// recordingHandler 收集日志，用来断言告警只记一次。
type recordingHandler struct {
	mu      sync.Mutex
	records []string
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r.Level >= slog.LevelWarn {
		h.records = append(h.records, r.Message)
	}
	return nil
}

func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler      { return h }

func (h *recordingHandler) warns() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.records)
}

func newTestViewCounter(w viewWriter) *viewCounter {
	return newViewCounter(w, ratelimit.NewMemory(), slog.New(slog.DiscardHandler))
}

// pendingOf 返回内存里待写库的计数，0 表示没有。
func pendingOf(v *viewCounter, id int64) int64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.pending[id]
}

func TestViewCounterRecord(t *testing.T) {
	reports := []struct {
		id int64
		ip string
	}{
		{7, "1.1.1.1"},
		{7, "1.1.1.1"}, // 同一 IP 同一内容，窗口内不重复计
		{7, "2.2.2.2"}, // 换 IP 再计一次
		{8, "1.1.1.1"}, // 同一 IP 换了内容，照计
		{0, "1.1.1.1"}, // 非法 ID 忽略
	}
	v := newTestViewCounter(newFakeViewWriter())
	for _, r := range reports {
		v.Record(r.id, r.ip)
	}
	if got := pendingOf(v, 7); got != 2 {
		t.Errorf("内容 7 的累计浏览量 = %d，期望 2", got)
	}
	if got := pendingOf(v, 8); got != 1 {
		t.Errorf("内容 8 的累计浏览量 = %d，期望 1", got)
	}
	if got := pendingOf(v, 0); got != 0 {
		t.Errorf("内容 0 不该被计数，实际 %d", got)
	}
}

func TestViewCounterWindowExpiry(t *testing.T) {
	v := newTestViewCounter(newFakeViewWriter())
	v.window = 0 // 窗口立即过期，模拟 30 分钟后再访问

	v.Record(3, "1.1.1.1")
	v.Record(3, "1.1.1.1")
	if got := pendingOf(v, 3); got != 2 {
		t.Errorf("窗口过期后同一 IP 应重新计数，累计 = %d，期望 2", got)
	}
}

func TestViewCounterFlush(t *testing.T) {
	w := newFakeViewWriter()
	v := newTestViewCounter(w)
	v.Record(5, "1.1.1.1")
	v.Record(5, "2.2.2.2")
	v.Record(6, "1.1.1.1")

	if err := v.Flush(context.Background()); err != nil {
		t.Fatalf("Flush 出错：%v", err)
	}
	if w.count(5) != 2 || w.count(6) != 1 {
		t.Fatalf("写库结果 = %v，期望 5:2、6:1", w.calls)
	}
	if got := pendingOf(v, 5); got != 0 {
		t.Errorf("Flush 后内容 5 应为空，实际 %d", got)
	}

	// 没有新的上报时不再写库
	if err := v.Flush(context.Background()); err != nil {
		t.Fatalf("空 Flush 出错：%v", err)
	}
	if w.count(5) != 2 {
		t.Errorf("空 Flush 不该重复写库，实际 %v", w.calls)
	}
}

func TestViewCounterFlushRetry(t *testing.T) {
	w := newFakeViewWriter(5)
	v := newTestViewCounter(w)
	v.Record(5, "1.1.1.1")
	v.Record(6, "1.1.1.1")

	if err := v.Flush(context.Background()); err == nil {
		t.Fatal("写库失败时 Flush 应返回错误")
	}
	if got := pendingOf(v, 5); got != 1 {
		t.Errorf("写失败的内容应放回重试，实际 %d", got)
	}
	if got := pendingOf(v, 6); got != 0 {
		t.Errorf("写成功的内容应清空，实际 %d", got)
	}

	// 恢复后连同新上报一起写入
	delete(w.fail, 5)
	v.Record(5, "2.2.2.2")
	if err := v.Flush(context.Background()); err != nil {
		t.Fatalf("重试 Flush 出错：%v", err)
	}
	if w.count(5) != 2 {
		t.Errorf("重试后内容 5 的写入量 = %d，期望 2", w.count(5))
	}
	if got := pendingOf(v, 5); got != 0 {
		t.Errorf("重试成功后应清空，实际 %d", got)
	}
}

// TestViewCounterIPLimit 同一 IP 每分钟最多计 60 次，超出的上报被丢弃。
func TestViewCounterIPLimit(t *testing.T) {
	// overLimit 的记录里，超过 60 次之后的 ID 都不该计数
	tests := []struct {
		name    string
		reports []int64 // 依次上报的内容 ID，同一个 IP
		want    map[int64]int64
	}{
		{
			name:    "第 60 次仍计数",
			reports: []int64{60},
			want:    map[int64]int64{60: 1},
		},
		{
			name:    "第 61 次被丢弃",
			reports: []int64{61},
			want:    map[int64]int64{},
		},
		{
			name:    "第 62 次也被丢弃",
			reports: []int64{62},
			want:    map[int64]int64{},
		},
		{
			name:    "换 IP 后重新计数",
			reports: []int64{63},
			want:    map[int64]int64{63: 1},
		},
	}
	v := newTestViewCounter(newFakeViewWriter())
	// 预热：同一 IP 先上报 60 次，把限流窗口用满
	for id := int64(1); id <= 60; id++ {
		v.Record(id, "1.1.1.1")
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := "1.1.1.1"
			if tt.name == "换 IP 后重新计数" {
				ip = "2.2.2.2"
			}
			for _, id := range tt.reports {
				v.Record(id, ip)
			}
			for id, want := range tt.want {
				if got := pendingOf(v, id); got != want {
					t.Errorf("内容 %d 的计数 = %d，期望 %d", id, got, want)
				}
			}
		})
	}
	// 被限流的上报不该建内容去重键：限流器里只有 60 个预热键加一个换 IP 的键
	if got := len(v.pending); got != 61 {
		t.Errorf("pending 条数 = %d，期望 61", got)
	}
}

// TestViewCounterPendingLimit pending 达到上限后丢弃新 ID，告警日志按间隔合并。
func TestViewCounterPendingLimit(t *testing.T) {
	logs := &recordingHandler{}
	v := newViewCounter(newFakeViewWriter(), ratelimit.NewMemory(), slog.New(logs))
	// 每个 IP 只上报一次，绕开 IP 限流，把 pending 填满
	for i := 0; i < viewPendingMax; i++ {
		v.Record(int64(i+1), fmt.Sprintf("10.%d.%d.%d", i/65536%256, i/256%256, i%256))
	}
	if got := len(v.pending); got != viewPendingMax {
		t.Fatalf("pending 条数 = %d，期望 %d", got, viewPendingMax)
	}

	// 已存在的内容继续累加，不受上限影响
	before := pendingOf(v, 1)
	v.Record(1, "192.0.2.1")
	if got := pendingOf(v, 1); got != before+1 {
		t.Errorf("已有内容应继续累加，实际 %d，期望 %d", got, before+1)
	}

	// 新 ID 被丢弃，连续丢弃只记一次告警
	for i := 0; i < 20; i++ {
		v.Record(int64(1<<40+i), "192.0.2.2")
	}
	if got := len(v.pending); got != viewPendingMax {
		t.Errorf("丢弃后 pending 条数 = %d，期望 %d", got, viewPendingMax)
	}
	if got := logs.warns(); got != 1 {
		t.Errorf("连续丢弃只应记一次告警，实际 %d 次", got)
	}
}

// TestViewCounterFlushBatch 单次刷库最多写 viewFlushBatch 条，剩下的留到下次。
func TestViewCounterFlushBatch(t *testing.T) {
	w := newFakeViewWriter()
	v := newTestViewCounter(w)
	total := viewFlushBatch + 100
	for i := 0; i < total; i++ {
		v.Record(int64(i+1), fmt.Sprintf("10.%d.%d.%d", i/65536%256, i/256%256, i%256))
	}

	if err := v.Flush(context.Background()); err != nil {
		t.Fatalf("第一次 Flush 出错：%v", err)
	}
	if got := len(w.calls); got != viewFlushBatch {
		t.Errorf("第一次刷库写入 %d 条，期望 %d 条", got, viewFlushBatch)
	}
	if err := v.Flush(context.Background()); err != nil {
		t.Fatalf("第二次 Flush 出错：%v", err)
	}
	if got := len(w.calls); got != total {
		t.Errorf("两次刷库共写入 %d 条，期望 %d 条", got, total)
	}
	if got := len(v.pending); got != 0 {
		t.Errorf("刷库后 pending 应为空，实际 %d 条", got)
	}
}
