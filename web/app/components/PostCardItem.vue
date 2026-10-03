<script setup lang="ts">
import type { PostCard } from '~/utils/content'

const props = withDefaults(defineProps<{ post: PostCard, flat?: boolean, compact?: boolean }>(), { flat: false, compact: false })
const version = computed(() => props.post.resource?.version)
</script>

<template>
  <NuxtLink
    :to="postUrl(post.id)"
    class="group flex flex-col overflow-hidden rounded-card bg-card transition duration-200 hover:-translate-y-[3px] hover:shadow-[0_10px_24px_-10px_rgba(38,42,49,.22)]"
    :class="flat ? 'border border-line' : 'shadow-card'"
  >
    <PostCover :post="post" pin />
    <span class="flex flex-1 flex-col gap-2 px-3.5 py-3 max-md:p-2.5">
      <b class="line-clamp-2 min-h-[3em] text-[15px] leading-normal font-semibold text-title group-hover:text-theme max-md:text-sm">
        {{ post.title }}<em v-if="version" class="ml-1 font-medium not-italic text-theme">v{{ version }}</em>
      </b>
      <span v-if="!compact" class="flex gap-1.5 overflow-hidden">
        <span class="rounded bg-theme-soft px-[7px] py-1 text-xs leading-none whitespace-nowrap text-theme">{{ post.category.name }}</span>
        <span v-for="t in post.tags.slice(0, 2)" :key="t.id" class="rounded bg-soft px-[7px] py-1 text-xs leading-none whitespace-nowrap text-muted"># {{ t.name }}</span>
      </span>
      <span class="mt-auto flex items-center gap-3 text-xs text-muted">
        <span v-if="!compact" class="inline-flex items-center gap-[3px]"><Icon name="clock" class="size-[13px]" />{{ relativeTime(post.publishedAt) }}</span>
        <span class="inline-flex items-center gap-[3px]" :class="!compact && 'max-md:hidden'"><Icon name="eye" class="size-[13px]" />{{ compactCount(post.viewCount) }}</span>
        <span v-if="post.resource?.fileSize" class="ml-auto">{{ post.resource.fileSize }}</span>
      </span>
    </span>
  </NuxtLink>
</template>
