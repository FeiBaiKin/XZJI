import type { PublicSchemas } from '@xzji/api-client/public'

export type PostCard = PublicSchemas['PostCard']
export type PostDetail = PublicSchemas['PostDetail']
export type CategoryNode = PublicSchemas['CategoryNode']
export type TagItem = PublicSchemas['Tag']
export type MembershipLevel = PublicSchemas['MembershipLevel']
export type HiddenBlockInfo = PublicSchemas['HiddenBlockInfo']
export type UnlockState = PublicSchemas['UnlockState']
export type Lock = PublicSchemas['Lock']
export type Image = PublicSchemas['Image']

// 图片的 srcset：有缩略图时按宽度列出让浏览器挑，没有就用原图
export function srcsetOf(image?: Image | null): string | undefined {
  return image?.variants?.length ? image.variants.map(v => `${v.url} ${v.width}w`).join(', ') : undefined
}

export const postUrl = (id: number) => `/${id}.html`
export const categoryUrl = (slug: string, page = 1) => page > 1 ? `/category/${slug}/page/${page}/` : `/category/${slug}/`
export const tagUrl = (slug: string, page = 1) => {
  const base = `/tag/${encodeURIComponent(slug)}/`
  return page > 1 ? `${base}page/${page}/` : base
}

// 分类颜色对应 main.css 里的渐变
export const gradientClass: Record<string, string> = {
  blue: 'bg-jb-blue',
  green: 'bg-jb-green',
  purple: 'bg-jb-purple',
  orange: 'bg-jb-orange',
  red: 'bg-jb-red',
  gold: 'bg-jb-gold text-[#5B3A00]',
  vip: 'bg-jb-vip text-vip',
}
export const gradientOf = (color?: string) => gradientClass[color ?? ''] ?? gradientClass.blue

// 没有封面和图标时，按内容 ID 取一个固定的底色
const palette = ['#EB7419', '#2D7FF0', '#1E9AB5', '#F2B321', '#23A55A', '#1F9D6B', '#6B5BE0', '#3A3F9C', '#C2410C', '#2F3A4A']
export const fallbackColor = (id: number) => palette[id % palette.length]!

// 标签云的配色，按位置轮换
export const tagColors = ['#2F6BFF', '#12B087', '#E8590C', '#7457F5', '#D6336C', '#0C8599', '#E8A317']

// 列表页的页码参数：非正整数一律视为第 1 页
export function pageParam(v: unknown): number {
  const n = Number(v)
  return Number.isSafeInteger(n) && n > 0 ? n : 1
}

export const yuan = (fen: number) => (fen / 100).toFixed(2)

// 解析解锁条件 member:N
export function memberRankOf(cond: string): number {
  const m = /^member:(\d+)$/.exec(cond)
  return m ? Number(m[1]) : 0
}

// 卡片右上角的角标：资源按下载区条件，文章只在设置了价格时显示
export function ribbonOf(post: Pick<PostCard, 'type' | 'price' | 'memberFreeRank' | 'resource'>, levelName: (rank: number) => string) {
  const cond = post.resource?.downloadCond
  if (post.type === 'article' || !cond) {
    return post.price > 0 ? { text: `¥${yuan(post.price)}`, class: 'bg-jb-red' } : null
  }
  if (cond === 'public') return { text: '免费', class: 'bg-jb-green' }
  if (cond === 'login') return { text: '登录下载', class: 'bg-jb-blue' }
  if (cond === 'comment') return { text: '评论下载', class: 'bg-jb-blue' }
  const rank = memberRankOf(cond)
  if (rank) return { text: `${levelName(rank)} 专享`, class: 'bg-jb-vip text-vip' }
  if (post.price > 0) return { text: `¥${yuan(post.price)}`, class: 'bg-jb-red' }
  if (post.memberFreeRank > 0) return { text: `${levelName(post.memberFreeRank)} 免费`, class: 'bg-jb-vip text-vip' }
  return null
}

// 未解锁时的提示语，cond 用于还没拿到解锁情况时（服务端渲染）
export function lockText(lock: Lock, levelName: (rank: number) => string, opts: { loggedIn: boolean, price: number, action: string }): string {
  switch (lock.reason) {
    case 'login':
      return `登录后可以${opts.action}`
    case 'comment':
      return opts.loggedIn ? `发表评论，审核通过后可以${opts.action}` : `登录并发表评论，审核通过后可以${opts.action}`
    case 'paid': {
      const parts = [opts.price > 0 ? `购买后可以${opts.action}` : `开通会员后可以${opts.action}`]
      if (lock.rank > 0) parts.push(`${levelName(lock.rank)} 及以上会员免费`)
      return parts.join('，')
    }
    case 'member':
      return `${levelName(lock.rank)} 及以上会员可以${opts.action}`
  }
}

// 由解锁条件推出默认的锁定原因，用于服务端渲染时的初始状态
export function lockOf(cond: string, memberFreeRank: number): Lock | null {
  if (cond === 'public') return null
  if (cond === 'login' || cond === 'comment') return { reason: cond, rank: 0 }
  if (cond === 'paid') return { reason: 'paid', rank: memberFreeRank }
  return { reason: 'member', rank: memberRankOf(cond) }
}
