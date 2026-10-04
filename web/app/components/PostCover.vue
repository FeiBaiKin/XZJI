<script setup lang="ts">
import type { PostCard } from '~/utils/content'

// 16:10 封面：有封面图用封面图；资源没有封面时，按图标主色配浅底，图标居中
const props = withDefaults(defineProps<{
  post: Pick<PostCard, 'id' | 'title' | 'cover' | 'resource' | 'pinned' | 'type' | 'price' | 'memberFreeRank'>
  pin?: boolean
  ribbon?: boolean
  tile?: number
  eager?: boolean
}>(), { pin: false, ribbon: false, tile: 64, eager: false })

const color = computed(() => props.post.resource?.icon?.color ?? fallbackColor(props.post.id))
const { levelName } = useLevelMap()
const tag = computed(() => props.ribbon ? ribbonOf(props.post, levelName) : null)
</script>

<template>
  <span class="relative grid aspect-[16/10] place-items-center overflow-hidden" :class="!post.cover && 'cover-tint'" :style="{ '--c': color }">
    <img
      v-if="post.cover"
      :src="post.cover.url"
      :alt="post.title"
      :width="post.cover.width"
      :height="post.cover.height"
      :loading="eager ? 'eager' : 'lazy'"
      class="absolute inset-0 size-full object-cover"
    >
    <AppTile v-else :post="post" :style="{ width: `${tile}px`, height: `${tile}px`, fontSize: `${tile}px` }" />
    <span v-if="tag" class="absolute top-0 right-0 rounded-bl-lg px-[9px] py-[5px] text-xs leading-none font-bold text-white" :class="tag.class">{{ tag.text }}</span>
    <span v-if="pin && post.pinned" class="absolute top-2 left-0 rounded-r bg-jb-red px-2 py-1 text-xs leading-none text-white">置顶</span>
    <span v-if="post.resource?.platform" class="absolute bottom-2 left-2 rounded bg-white/85 px-[7px] py-1 text-xs leading-none text-title">{{ post.resource.platform }}</span>
  </span>
</template>
