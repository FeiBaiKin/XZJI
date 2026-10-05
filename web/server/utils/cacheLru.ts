// Nitro 路由缓存的默认 memory 驱动只存不淘汰，而缓存键包含完整查询串（Nitro 的 getKey 用
// req.originalUrl 做哈希），随机参数 ?x=1、?x=2……每来一个就是一个新条目，请求量一大前台
// 进程的内存就被撑爆。
//
// 上限依据：M5 压测实测前台常驻内存 107.4MB，目标是不超过 200MB；单页 HTML 约 50KB，
// 500 条约 25MB，加上固定开销仍留有余量。单条响应超过字节上限时直接不缓存（宁可这次
// 不缓存，也不让内存没有上限）。Nitro 自己按 maxAge 判断过期，这里只负责容量淘汰。

export interface LruCacheOptions {
  // 条目上限
  maxEntries?: number
  // 总字节上限
  maxBytes?: number
}

const defaultMaxEntries = 500
const defaultMaxBytes = 64 * 1024 * 1024

// 每条缓存的固定开销估算：键、Map 记录、字符串头等
const entryOverhead = 64

// 前台没有 Node 类型定义，用 TextEncoder 数 UTF-8 字节（缓存的 HTML 含中文，不能按字符数算）
const encoder = new TextEncoder()

interface Entry {
  value: string
  size: number
}

// createLruCache 一个按最近使用淘汰的字符串存储。
export function createLruCache(options: LruCacheOptions = {}) {
  const maxEntries = options.maxEntries && options.maxEntries > 0 ? options.maxEntries : defaultMaxEntries
  const maxBytes = options.maxBytes && options.maxBytes > 0 ? options.maxBytes : defaultMaxBytes
  // Map 保持插入顺序，第一个键就是最久未使用的
  const items = new Map<string, Entry>()
  let bytes = 0

  function entrySize(key: string, value: string): number {
    return entryOverhead + encoder.encode(key).length + encoder.encode(value).length
  }

  function touch(key: string): Entry | undefined {
    const entry = items.get(key)
    if (entry) {
      // 重新插入把它移到队尾，实现最近使用
      items.delete(key)
      items.set(key, entry)
    }
    return entry
  }

  function evict(): void {
    while (items.size > maxEntries || bytes > maxBytes) {
      const oldest = items.keys().next().value
      if (oldest === undefined) return
      remove(oldest)
    }
  }

  function remove(key: string): void {
    const entry = items.get(key)
    if (!entry) return
    items.delete(key)
    bytes -= entry.size
  }

  return {
    // size 和 bytes 写成函数：驱动对象是展开（spread）出来的，取值器会被提前求值
    size: (): number => items.size,
    bytes: (): number => bytes,
    getItem(key: string): string | null {
      return touch(key)?.value ?? null
    },
    hasItem(key: string): boolean {
      return items.has(key)
    },
    setItem(key: string, value: string): void {
      const size = entrySize(key, value)
      remove(key)
      if (size > maxBytes) return
      items.set(key, { value, size })
      bytes += size
      evict()
    },
    removeItem(key: string): void {
      remove(key)
    },
    getKeys(base = ''): string[] {
      const keys = [...items.keys()]
      return base ? keys.filter(key => key.startsWith(base)) : keys
    },
    clear(base = ''): void {
      if (!base) {
        items.clear()
        bytes = 0
        return
      }
      for (const key of [...items.keys()]) {
        if (key.startsWith(base)) remove(key)
      }
    },
  }
}

// createLruDriver 包装成 unstorage 驱动，挂到 cache 挂载点上。
export function createLruDriver(options: LruCacheOptions = {}) {
  const cache = createLruCache(options)
  return {
    name: 'xzji-lru',
    options,
    ...cache,
  }
}
