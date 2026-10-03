const pad = (n: number) => String(n).padStart(2, '0')

// 列表里的发布时间：一天内显示几小时前，一周内显示几天前，再早显示日期
export function relativeTime(iso: string, now = Date.now()): string {
  const t = new Date(iso).getTime()
  const diff = Math.max(0, now - t)
  const hour = 3600e3
  if (diff < hour) return '刚刚'
  if (diff < 24 * hour) return `${Math.floor(diff / hour)} 小时前`
  if (diff < 48 * hour) return '昨天'
  if (diff < 7 * 24 * hour) return `${Math.floor(diff / (24 * hour))} 天前`
  const d = new Date(t)
  return d.getFullYear() === new Date(now).getFullYear()
    ? `${d.getMonth() + 1}月${d.getDate()}日`
    : `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

export function dateOnly(iso: string): string {
  const d = new Date(iso)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

// 计数：一万以上显示为“1.2万”，以下加千分位
export function compactCount(n: number): string {
  if (n >= 10000) return `${(n / 10000).toFixed(1).replace(/\.0$/, '')}万`
  return n.toLocaleString('en-US')
}
