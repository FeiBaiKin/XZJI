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
	// DefaultMaxBytes 缓存占用的总字节上限。单条响应最多 1MB（见 httpapi.maxCacheBody），
	// 只限条数时最坏能占到 GB 级；后端常驻内存目标是 100MB 以内，留 32MB 给响应缓存，
	// 其余留给数据库连接、图片处理等瞬时开销。
	DefaultMaxBytes = 32 << 20
	// DefaultTTL 兜底过期时间：清除通知没送到时，最多这么久也会重新取一次
	DefaultTTL = 10 * time.Minute
)

// entryOverhead 每条缓存的固定开销估算：list 元素、map 记录、过期时间等。
const entryOverhead = 64

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
	mu       sync.Mutex
	max      int
	maxBytes int64
	ttl      time.Duration
	now      func() time.Time
	bytes    int64
	items    map[string]*list.Element
	// order 最近使用的在队首
	order *list.List
}

// Option 调整缓存的容量参数。
type Option func(*LRU)

// WithMaxBytes 设置总字节上限，非正数时取默认值。
func WithMaxBytes(n int64) Option {
	return func(l *LRU) {
		if n > 0 {
			l.maxBytes = n
		}
	}
}

// NewLRU 参数非正数时取默认值。条数和字节两个上限同时生效。
func NewLRU(size int, ttl time.Duration, opts ...Option) *LRU {
	if size <= 0 {
		size = DefaultSize
	}
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	l := &LRU{max: size, maxBytes: DefaultMaxBytes, ttl: ttl, now: time.Now, items: make(map[string]*list.Element), order: list.New()}
	for _, opt := range opts {
		opt(l)
	}
	return l
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
		l.bytes += int64(size(key, e) - size(key, it.entry))
		it.entry, it.expires = e, expires
		l.order.MoveToFront(el)
		l.evictLocked()
		return
	}
	l.items[key] = l.order.PushFront(&item{key: key, entry: e, expires: expires})
	l.bytes += int64(size(key, e))
	l.evictLocked()
}

// evictLocked 从最久未使用的开始淘汰，直到条数和字节都不超上限。单个条目本身就超过
// 字节上限时会把自己也淘汰掉，宁可这次不缓存，也不让内存无上限增长。
func (l *LRU) evictLocked() {
	for l.order.Len() > l.max || l.bytes > l.maxBytes {
		el := l.order.Back()
		if el == nil {
			return
		}
		l.removeLocked(el)
	}
}

func (l *LRU) Purge() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.items = make(map[string]*list.Element)
	l.order.Init()
	l.bytes = 0
}

// size 估算一条缓存占用的字节数。
func size(key string, e Entry) int {
	return entryOverhead + len(key) + len(e.ContentType) + len(e.Body)
}

func (l *LRU) removeLocked(el *list.Element) {
	if el == nil {
		return
	}
	it := el.Value.(*item)
	l.order.Remove(el)
	delete(l.items, it.key)
	l.bytes -= int64(size(it.key, it.entry))
}
