<script setup lang="ts">
import type { PostCard } from '~/utils/content'

// 文章列表项：左侧缩略图，右侧标题、摘要和发布信息
const props = defineProps<{ post: PostCard }>()
const { byId } = useCategoryMap()
const category = computed(() => byId(props.post.category.id))
</script>

<template>
  <NuxtLink :to="postUrl(post.id)" class="group grid grid-cols-[168px_minmax(0,1fr)] gap-4 border-b border-line py-4 last:border-b-0 max-md:grid-cols-[110px_minmax(0,1fr)] max-md:gap-3">
    <span class="relative grid aspect-[16/10] place-items-center overflow-hidden rounded-lg text-white" :class="!post.cover && gradientOf(category?.color)">
      <img v-if="post.cover" :src="post.cover.url" :srcset="srcsetOf(post.cover)" sizes="220px" :alt="post.title" :width="post.cover.width" :height="post.cover.height" loading="lazy" class="absolute inset-0 size-full object-cover">
      <Icon v-else :name="category?.icon || 'book'" class="size-10 stroke-[1.7]" />
      <span class="absolute top-1.5 left-1.5 rounded bg-black/25 px-1.5 py-1 text-[11.5px] leading-none text-white">{{ post.category.name }}</span>
    </span>
    <span class="min-w-0">
      <b class="block text-base leading-normal text-title group-hover:text-theme">{{ post.title }}</b>
      <p class="mt-1 line-clamp-2 text-[13.5px] text-muted max-md:hidden">{{ post.summary }}</p>
      <span class="mt-2.5 flex items-center gap-3 text-xs text-muted">
        <span class="inline-flex items-center gap-1.5">
          <span class="grid size-5 place-items-center rounded-full bg-jb-blue text-[11px] font-bold text-white">{{ post.author.nickname.slice(0, 1) }}</span>
          <span class="max-md:hidden">{{ post.author.nickname }}</span>
        </span>
        <span class="inline-flex items-center gap-[3px]"><Icon name="clock" class="size-[13px]" />{{ relativeTime(post.publishedAt) }}</span>
        <span class="inline-flex items-center gap-[3px]"><Icon name="eye" class="size-[13px]" />{{ compactCount(post.viewCount) }}</span>
        <span v-if="post.commentCount" class="inline-flex items-center gap-[3px]"><Icon name="chat" class="size-[13px]" />{{ post.commentCount }}</span>
      </span>
    </span>
  </NuxtLink>
</template>
