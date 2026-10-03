<script setup lang="ts">
import type { PostCard } from '~/utils/content'

defineProps<{ posts: PostCard[] }>()
</script>

<template>
  <ol class="px-4 pb-3">
    <li v-for="p in posts" :key="p.id" class="flex items-center gap-2.5 py-2 [&+&]:border-t [&+&]:border-line">
      <span class="rank-no">{{ posts.indexOf(p) + 1 }}</span>
      <NuxtLink :to="postUrl(p.id)" class="group flex min-w-0 flex-1 items-center gap-2.5">
        <AppTile :post="p" class="size-[34px] rounded-[9px] text-[34px] shadow-none" />
        <span class="min-w-0">
          <b class="block truncate text-sm leading-[1.35] font-semibold text-title group-hover:text-theme">{{ p.title }}</b>
          <small class="text-xs text-muted">浏览 {{ compactCount(p.viewCount) }} 次</small>
        </span>
      </NuxtLink>
    </li>
  </ol>
</template>
