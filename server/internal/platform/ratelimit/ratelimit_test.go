package ratelimit

import (
	"testing"
	"time"
)

func TestMemoryWindow(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	l := NewMemory()
	l.now = func() time.Time { return now }

	for i := 1; i <= 3; i++ {
		if n, _ := l.Incr("k", time.Minute); n != i {
			t.Fatalf("第 %d 次计数得到 %d", i, n)
		}
	}
	if n, retry := l.Count("k"); n != 3 || retry != time.Minute {
		t.Fatalf("Count = %d, %v", n, retry)
	}

	now = now.Add(59 * time.Second)
	if n, retry := l.Incr("k", time.Minute); n != 4 || retry != time.Second {
		t.Fatalf("窗口内继续累计：%d, %v", n, retry)
	}

	now = now.Add(time.Second)
	if n, _ := l.Count("k"); n != 0 {
		t.Fatalf("窗口结束后应清零，得到 %d", n)
	}
	if n, _ := l.Incr("k", time.Minute); n != 1 {
		t.Fatalf("新窗口从 1 开始，得到 %d", n)
	}

	l.Reset("k")
	if n, _ := l.Count("k"); n != 0 {
		t.Fatalf("Reset 后应为 0，得到 %d", n)
	}
}
