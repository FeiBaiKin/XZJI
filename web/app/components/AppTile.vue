<script setup lang="ts">
import type { PostCard } from '~/utils/content'

// 软件图标块：有图标用图标，没有就用标题首字
const props = defineProps<{ post: Pick<PostCard, 'id' | 'title' | 'resource'> }>()
const icon = computed(() => props.post.resource?.icon)
const color = computed(() => icon.value?.color ?? fallbackColor(props.post.id))
</script>

<template>
  <span class="app-tile" :style="{ '--c': color }">
    <img v-if="icon" :src="icon.url" :srcset="srcsetOf(icon)" sizes="96px" alt="" :width="icon.width" :height="icon.height" loading="lazy" class="size-full object-cover">
    <span v-else class="text-[.45em] font-bold leading-none" aria-hidden="true">{{ post.title.slice(0, 1) }}</span>
  </span>
</template>
