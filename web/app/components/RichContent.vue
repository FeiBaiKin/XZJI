<script setup lang="ts">
import type { HiddenBlockInfo, UnlockState } from '~/utils/content'

// 正文：按隐藏块占位符切开，公开部分直接渲染，占位符位置换成隐藏内容框
const props = defineProps<{ html: string, blocks: HiddenBlockInfo[], state: UnlockState | null, memberFreeRank: number }>()
const emit = defineEmits<{ buy: [] }>()

const segments = computed(() => {
  const out: ({ html: string } | { info: HiddenBlockInfo })[] = []
  const re = /<div data-hidden-block="(\d+)"><\/div>/g
  let last = 0
  for (const m of props.html.matchAll(re)) {
    if (m.index > last) out.push({ html: props.html.slice(last, m.index) })
    const info = props.blocks.find(b => b.seq === Number(m[1]))
    if (info) out.push({ info })
    last = m.index + m[0].length
  }
  if (last < props.html.length) out.push({ html: props.html.slice(last) })
  return out
})
</script>

<template>
  <div class="prose-x">
    <template v-for="(seg, i) in segments" :key="i">
      <!-- 正文已在后端按白名单过滤，只含公开内容 -->
      <div v-if="'html' in seg" v-html="seg.html" />
      <HiddenBox v-else :info="seg.info" :state="state" :member-free-rank="memberFreeRank" @buy="emit('buy')" />
    </template>
  </div>
</template>
