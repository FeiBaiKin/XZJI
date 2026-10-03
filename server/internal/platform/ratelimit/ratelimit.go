// Package ratelimit 提供按 key 计数的固定窗口限流。第一阶段单实例部署用内存实现，
// 多实例部署时实现同一接口的 Redis 版本即可替换。
package ratelimit

import (
	"context"
	"sync"
	"time"
)

type Limiter interface {
	// Incr 计数加一，返回当前窗口内的累计次数和窗口剩余时间。
	Incr(key string, window time.Duration) (count int, retryAfter time.Duration)
	// Count 返回当前窗口内的累计次数。
	Count(key string) (count int, retryAfter time.Duration)
	Reset(key string)
}

type entry struct {
	count int
	reset time.Time
}

type Memory struct {
	mu  sync.Mutex
	m   map[string]*entry
	now func() time.Time
}

func NewMemory() *Memory {
	return &Memory{m: make(map[string]*entry), now: time.Now}
}

func (l *Memory) Incr(key string, window time.Duration) (int, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e, ok := l.m[key]
	if !ok || !now.Before(e.reset) {
		e = &entry{reset: now.Add(window)}
		l.m[key] = e
	}
	e.count++
	return e.count, e.reset.Sub(now)
}

func (l *Memory) Count(key string) (int, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e, ok := l.m[key]
	if !ok || !now.Before(e.reset) {
		return 0, 0
	}
	return e.count, e.reset.Sub(now)
}

func (l *Memory) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.m, key)
}

// Sweep 定期清理过期的计数，防止内存只增不减，ctx 取消后退出。
func (l *Memory) Sweep(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			l.mu.Lock()
			now := l.now()
			for k, e := range l.m {
				if !now.Before(e.reset) {
					delete(l.m, k)
				}
			}
			l.mu.Unlock()
		}
	}
}
