// Package cache 缓存公开接口的响应。第一阶段是单实例部署，用进程内的 LRU 实现；
// 多实例部署时实现同一接口的 Redis 版本即可替换（见总文档第 14 节）。
package cache

import (
	"container/list"
	"sync"
	"time"
)

const (
	// DefaultSize 缓存条目上限，超过后淘汰最久未使用的
	DefaultSize = 2000
	// DefaultTTL 兜底过期时间：清除通知没送到时，最多这么久也会重新取一次
	DefaultTTL = 10 * time.Minute
)

// Entry 缓存的 HTTP 响应。
type Entry struct {
	Status      int
	ContentType string
	Body        []byte
}

type Cache interface {
	// Get 取出未过期的缓存，不存在或已过期时返回 false。
	Get(key string) (Entry, bool)
	Set(key string, e Entry)
	// Purge 清空全部缓存，内容或设置变更后调用。
	Purge()
}

type item struct {
	key     string
	entry   Entry
	expires time.Time
}

// LRU 进程内缓存：容量固定，按最近使用淘汰，每条带过期时间。
type LRU struct {
	mu    sync.Mutex
	max   int
	ttl   time.Duration
	now   func() time.Time
	items map[string]*list.Element
	// order 最近使用的在队首
	order *list.List
}

// NewLRU 参数非正数时取默认值。
func NewLRU(size int, ttl time.Duration) *LRU {
	if size <= 0 {
		size = DefaultSize
	}
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &LRU{max: size, ttl: ttl, now: time.Now, items: make(map[string]*list.Element), order: list.New()}
}

func (l *LRU) Get(key string) (Entry, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	el, ok := l.items[key]
	if !ok {
		return Entry{}, false
	}
	it := el.Value.(*item)
	if !l.now().Before(it.expires) {
		l.removeLocked(el)
		return Entry{}, false
	}
	l.order.MoveToFront(el)
	return it.entry, true
}

func (l *LRU) Set(key string, e Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	expires := l.now().Add(l.ttl)
	if el, ok := l.items[key]; ok {
		it := el.Value.(*item)
		it.entry, it.expires = e, expires
		l.order.MoveToFront(el)
		return
	}
	l.items[key] = l.order.PushFront(&item{key: key, entry: e, expires: expires})
	for l.order.Len() > l.max {
		l.removeLocked(l.order.Back())
	}
}

func (l *LRU) Purge() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.items = make(map[string]*list.Element)
	l.order.Init()
}

func (l *LRU) removeLocked(el *list.Element) {
	if el == nil {
		return
	}
	l.order.Remove(el)
	delete(l.items, el.Value.(*item).key)
}
