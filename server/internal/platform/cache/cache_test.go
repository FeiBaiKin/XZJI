package cache

import (
	"testing"
	"time"
)

func TestNewLRUDefaults(t *testing.T) {
	l := NewLRU(0, 0)
	if l.max != DefaultSize || l.ttl != DefaultTTL {
		t.Errorf("参数非正数时应取默认值，实际 size=%d ttl=%v", l.max, l.ttl)
	}
}

func TestLRUGetSet(t *testing.T) {
	l := NewLRU(10, time.Minute)
	if _, ok := l.Get("missing"); ok {
		t.Error("不存在的键不应命中")
	}
	want := Entry{Status: 200, ContentType: "application/json", Body: []byte(`{"a":1}`)}
	l.Set("k", want)
	got, ok := l.Get("k")
	if !ok {
		t.Fatal("刚写入的键应命中")
	}
	if got.Status != want.Status || got.ContentType != want.ContentType || string(got.Body) != string(want.Body) {
		t.Errorf("取出的内容 = %+v，期望 %+v", got, want)
	}
}

func TestLRUExpiry(t *testing.T) {
	now := time.Unix(1000, 0)
	l := NewLRU(10, time.Minute)
	l.now = func() time.Time { return now }
	l.Set("k", Entry{Status: 200})

	now = now.Add(time.Minute - time.Second)
	if _, ok := l.Get("k"); !ok {
		t.Error("未过期时不应失效")
	}
	now = now.Add(2 * time.Second)
	if _, ok := l.Get("k"); ok {
		t.Error("超过 TTL 后应失效")
	}
	// 过期条目已被清掉，不会一直占着内存
	if l.order.Len() != 0 {
		t.Errorf("过期条目应被删除，实际还有 %d 条", l.order.Len())
	}
}

func TestLRUEviction(t *testing.T) {
	l := NewLRU(2, time.Minute)
	l.Set("a", Entry{Status: 200})
	l.Set("b", Entry{Status: 200})
	// 读一次 a，让它比 b 新
	if _, ok := l.Get("a"); !ok {
		t.Fatal("a 应命中")
	}
	l.Set("c", Entry{Status: 200})

	if _, ok := l.Get("b"); ok {
		t.Error("最久未使用的 b 应被淘汰")
	}
	if _, ok := l.Get("a"); !ok {
		t.Error("刚访问过的 a 不应被淘汰")
	}
	if _, ok := l.Get("c"); !ok {
		t.Error("新写入的 c 不应被淘汰")
	}
}

func TestLRUPurge(t *testing.T) {
	l := NewLRU(10, time.Minute)
	l.Set("a", Entry{Status: 200})
	l.Set("b", Entry{Status: 200})
	l.Purge()
	if _, ok := l.Get("a"); ok {
		t.Error("Purge 后不应命中")
	}
	if l.order.Len() != 0 {
		t.Errorf("Purge 后应为空，实际还有 %d 条", l.order.Len())
	}
	// 清空后还能继续正常写入
	l.Set("c", Entry{Status: 200})
	if _, ok := l.Get("c"); !ok {
		t.Error("Purge 后写入的键应命中")
	}
}
