<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'

const props = defineProps<{ page: number, total: number, pageSize: number, to: (page: number) => RouteLocationRaw }>()
const pages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))

// 页码：首页、末页和当前页前后各两页，中间用省略号
const items = computed(() => {
  const n = pages.value
  const set = new Set([1, n, ...[-2, -1, 0, 1, 2].map(d => props.page + d)].filter(p => p >= 1 && p <= n))
  const sorted = [...set].sort((a, b) => a - b)
  const out: (number | '…')[] = []
  sorted.forEach((p, i) => {
    if (i > 0 && p - sorted[i - 1]! > 1) out.push('…')
    out.push(p)
  })
  return out
})

const jump = ref(String(props.page))
watch(() => props.page, (p) => { jump.value = String(p) })
function go() {
  const p = Math.min(pages.value, Math.max(1, Math.floor(Number(jump.value)) || 1))
  navigateTo(props.to(p))
}

const cell = 'grid h-9 min-w-9 place-items-center rounded-md px-2.5'
</script>

<template>
  <nav v-if="pages > 1" class="mt-6 flex flex-wrap items-center justify-center gap-1.5 text-[13.5px]" aria-label="分页">
    <NuxtLink v-if="page > 1" :to="to(page - 1)" :class="cell" class="bg-card shadow-card hover:text-theme">上一页</NuxtLink>
    <template v-for="(p, i) in items" :key="i">
      <span v-if="p === '…'" :class="cell" class="text-muted">…</span>
      <NuxtLink
        v-else
        :to="to(p)"
        :class="[cell, p === page ? 'bg-jb-blue text-white' : 'bg-card shadow-card hover:text-theme']"
        :aria-current="p === page ? 'page' : undefined"
      >
        {{ p }}
      </NuxtLink>
    </template>
    <NuxtLink v-if="page < pages" :to="to(page + 1)" :class="cell" class="bg-card shadow-card hover:text-theme">下一页</NuxtLink>
    <form class="ml-2 flex items-center gap-1.5 text-muted max-md:hidden" @submit.prevent="go">
      跳到 <input v-model="jump" inputmode="numeric" aria-label="页码" class="h-8 w-12 rounded-[5px] border border-field text-center text-title outline-none focus:border-theme"> 页
    </form>
  </nav>
</template>
