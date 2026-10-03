import type { PublicSchemas } from '@xzji/api-client/public'

export type PostCard = PublicSchemas['PostCard']
export type PostDetail = PublicSchemas['PostDetail']
export type CategoryNode = PublicSchemas['CategoryNode']
export type TagItem = PublicSchemas['Tag']

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
