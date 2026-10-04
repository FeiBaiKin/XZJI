package frontsync

import (
	"io"
	"log/slog"
	"testing"

	"xzji/internal/platform/cache"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestSignature(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		ts     int64
		want   string
	}{
		{"常规", "test-secret", 1700000000, "1b74eaf87da46877ba651ac812b24a3bf01990e9ed0101b88268f60f965729e2"},
		{"空密钥", "", 0, "9979e4c3ee19965f9ecc6dca6b3954b10c18c1bfa105c512cc3ae58be85db71c"},
		{"非 ASCII 密钥", "s3cr3t-中文", 1759500000, "dae49e7df402bfc921fd9ce5341ae77dccad92d886aca9d0ea16dfa9a14be697"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Signature(tt.secret, tt.ts); got != tt.want {
				t.Errorf("Signature(%q, %d) = %s，期望 %s", tt.secret, tt.ts, got, tt.want)
			}
		})
	}
}

// 后端缓存必须在通知前台之前清掉，否则前台会拿旧数据重新渲染。
func TestPurgeClearsBackendCache(t *testing.T) {
	store := cache.NewLRU(10, 0)
	store.Set("key", cache.Entry{Status: 200, Body: []byte("老数据")})
	// 不配前台地址时只清后端缓存
	s := NewService(store, nil, "secret", testLogger())
	s.Purge()
	if _, ok := store.Get("key"); ok {
		t.Error("Purge 后后端缓存应被清空")
	}
}
