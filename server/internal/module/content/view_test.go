package content

import (
	"context"
	"errors"
	"testing"

	"xzji/internal/dbgen"
	"xzji/internal/platform/ratelimit"
)

// fakeViewWriter 累计写入的计数，fail 里的内容一直写失败。
type fakeViewWriter struct {
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
	if f.fail[arg.ID] {
		return errors.New("写库失败")
	}
	f.calls[arg.ID] += arg.ViewCount
	return nil
}

func newTestViewCounter(w viewWriter) *viewCounter {
	return newViewCounter(w, ratelimit.NewMemory())
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
	if w.calls[5] != 2 || w.calls[6] != 1 {
		t.Fatalf("写库结果 = %v，期望 5:2、6:1", w.calls)
	}
	if got := pendingOf(v, 5); got != 0 {
		t.Errorf("Flush 后内容 5 应为空，实际 %d", got)
	}

	// 没有新的上报时不再写库
	if err := v.Flush(context.Background()); err != nil {
		t.Fatalf("空 Flush 出错：%v", err)
	}
	if w.calls[5] != 2 {
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
	if w.calls[5] != 2 {
		t.Errorf("重试后内容 5 的写入量 = %d，期望 2", w.calls[5])
	}
	if got := pendingOf(v, 5); got != 0 {
		t.Errorf("重试成功后应清空，实际 %d", got)
	}
}
