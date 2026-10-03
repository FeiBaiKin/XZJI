import { ref } from 'vue'
import type { AdminSchemas } from '@xzji/api-client/admin'
import sprite from '../../web/public/icons.svg?raw'
import { api } from './api'

export type Category = AdminSchemas['AdminCategory']
export type CategoryNode = Category & { children: CategoryNode[] }

// 前台图标库（web/public/icons.svg），后台挂载同一份，选分类图标时可以直接预览
let spriteMounted = false
export function mountIconSprite() {
  if (spriteMounted) return
  spriteMounted = true
  const holder = document.createElement('div')
  holder.style.display = 'none'
  holder.innerHTML = sprite
  document.body.prepend(holder)
}

export const iconNames = [...sprite.matchAll(/id="i-([a-z0-9-]+)"/g)].map(m => m[1]!)

// 与前台 Tailwind 主题里的渐变一致
export const gradients: Record<string, { label: string, css: string }> = {
  blue: { label: '蓝', css: 'linear-gradient(135deg,#5AA9FF 0%,#2F6BFF 100%)' },
  green: { label: '绿', css: 'linear-gradient(135deg,#4FD8A8 0%,#12B087 100%)' },
  purple: { label: '紫', css: 'linear-gradient(135deg,#B48CFF 0%,#7457F5 100%)' },
  orange: { label: '橙', css: 'linear-gradient(135deg,#FFC35A 0%,#FF8A1F 100%)' },
  red: { label: '红', css: 'linear-gradient(135deg,#FF8A6B 10%,#FF3B3B 100%)' },
  gold: { label: '金', css: 'linear-gradient(135deg,#FCE3B4 0%,#E8B65E 100%)' },
  vip: { label: '黑金', css: 'linear-gradient(300deg,#4C4D51,#2A2A31 15%,#85858A 40%,#393A3C 60%,#393838 80%,#5E5F62)' },
}

export function buildTree(list: Category[]): CategoryNode[] {
  const nodes = new Map(list.map(c => [c.id, { ...c, children: [] as CategoryNode[] }]))
  const roots: CategoryNode[] = []
  for (const c of list) {
    const node = nodes.get(c.id)!
    const parent = c.parentId ? nodes.get(c.parentId) : undefined
    if (parent) parent.children.push(node)
    else roots.push(node)
  }
  return roots
}

// 分类列表在多个页面共用，修改分类后调用 loadCategories(true) 刷新
export const categories = ref<Category[]>([])
let loaded: Promise<void> | null = null
export function loadCategories(force = false) {
  if (force || !loaded) {
    loaded = api.GET('/categories').then(({ data }) => {
      if (data) categories.value = data.items
      else loaded = null
    })
  }
  return loaded
}

export const typeLabel: Record<AdminSchemas['PostType'], string> = { article: '文章', resource: '资源' }
