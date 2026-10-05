package cache

import (
	"testing"
	"time"
)

func TestNewLRUDefaults(t *testing.T) {
	l := NewLRU(0, 0)
	if l.max != DefaultSize || l.maxBytes != DefaultMaxBytes || l.ttl != DefaultTTL {
		t.Errorf("参数非正数时应取默认值，实际 size=%d maxBytes=%d ttl=%v", l.max, l.maxBytes, l.ttl)
	}
	// 非正数的字节上限同样退回默认值
	if got := NewLRU(10, time.Minute, WithMaxBytes(0)); got.maxBytes != DefaultMaxBytes {
		t.Errorf("WithMaxBytes(0) 应取默认值，实际 %d", got.maxBytes)
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
	if l.bytes != 0 {
		t.Errorf("Purge 后字节数应归零，实际 %d", l.bytes)
	}
	// 清空后还能继续正常写入
	l.Set("c", Entry{Status: 200})
	if _, ok := l.Get("c"); !ok {
		t.Error("Purge 后写入的键应命中")
	}
}

// body 生成指定字节数的响应体。
func body(n int) []byte { return make([]byte, n) }

func TestLRUByteLimit(t *testing.T) {
	// 单条占用 = entryOverhead + len(key) + len(ContentType) + len(body)
	tests := []struct {
		name     string
		maxBytes int64
		keys     []string // 依次写入，写入后各读一次，保证访问顺序
		hit      []string
		miss     []string
	}{
		{
			// 每条 64+1+16+100 = 181 字节，上限 400 只能放下两条
			name:     "按字节淘汰最久未使用的",
			maxBytes: 400,
			keys:     []string{"a", "b", "c"},
			hit:      []string{"b", "c"},
			miss:     []string{"a"},
		},
		{
			name:     "上限只够一条时只留最新的一条",
			maxBytes: 200,
			keys:     []string{"a", "b"},
			hit:      []string{"b"},
			miss:     []string{"a"},
		},
		{
			name:     "字节上限放宽后全部保留",
			maxBytes: 1000,
			keys:     []string{"a", "b", "c"},
			hit:      []string{"a", "b", "c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLRU(100, time.Minute, WithMaxBytes(tt.maxBytes))
			for _, k := range tt.keys {
				l.Set(k, Entry{Status: 200, ContentType: "application/json", Body: body(100)})
			}
			if l.bytes > tt.maxBytes {
				t.Errorf("占用字节 %d 超过上限 %d", l.bytes, tt.maxBytes)
			}
			for _, k := range tt.hit {
				if _, ok := l.Get(k); !ok {
					t.Errorf("%q 应命中", k)
				}
			}
			for _, k := range tt.miss {
				if _, ok := l.Get(k); ok {
					t.Errorf("%q 应已淘汰", k)
				}
			}
		})
	}
}

func TestLRUByteLimitWithSizeLimit(t *testing.T) {
	// 条数上限先到：字节还有富余，也只留 size 条
	l := NewLRU(2, time.Minute, WithMaxBytes(1<<20))
	for _, k := range []string{"a", "b", "c"} {
		l.Set(k, Entry{Status: 200, Body: body(10)})
	}
	if l.order.Len() != 2 {
		t.Errorf("条数上限为 2，实际 %d 条", l.order.Len())
	}
	if _, ok := l.Get("a"); ok {
		t.Error("超出条数上限时最久未使用的 a 应被淘汰")
	}
}

func TestLRUEntryLargerThanLimit(t *testing.T) {
	l := NewLRU(10, time.Minute, WithMaxBytes(300))
	l.Set("big", Entry{Status: 200, ContentType: "application/json", Body: body(4096)})
	if _, ok := l.Get("big"); ok {
		t.Error("单条超过字节上限时不应保留")
	}
	if l.bytes != 0 || l.order.Len() != 0 {
		t.Errorf("不应留下任何占用，实际 bytes=%d len=%d", l.bytes, l.order.Len())
	}
	// 超过上限的条目被淘汰后，缓存还能继续用
	l.Set("small", Entry{Status: 200, Body: body(10)})
	if _, ok := l.Get("small"); !ok {
		t.Error("写入小响应后应命中")
	}
}

func TestLRUResizeAccounting(t *testing.T) {
	l := NewLRU(10, time.Minute, WithMaxBytes(1<<20))
	l.Set("k", Entry{Status: 200, ContentType: "application/json", Body: body(1000)})
	one := l.bytes
	// 同一个键改小：字节数跟着变小，不会把别的条目挤掉
	l.Set("k", Entry{Status: 200, ContentType: "application/json", Body: body(10)})
	if l.bytes != one-990 {
		t.Errorf("改写后字节数 = %d，期望 %d", l.bytes, one-990)
	}
	l.Set("other", Entry{Status: 200, Body: body(10)})
	if _, ok := l.Get("k"); !ok {
		t.Error("改写大条目不应导致自己被淘汰")
	}
	if want := int64(size("k", Entry{Status: 200, ContentType: "application/json", Body: body(10)}) + size("other", Entry{Status: 200, Body: body(10)})); l.bytes != want {
		t.Errorf("字节统计不准：%d，期望 %d", l.bytes, want)
	}
}
