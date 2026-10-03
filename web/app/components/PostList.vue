<script setup lang="ts">
import type { PostCard } from '~/utils/content'

// 全是文章时用图文列表，有资源时用卡片网格
const props = withDefaults(defineProps<{ posts: PostCard[], empty?: string }>(), { empty: '这里还没有内容' })
const asList = computed(() => props.posts.length > 0 && props.posts.every(p => p.type === 'article'))
</script>

<template>
  <div v-if="!posts.length" class="card grid place-items-center gap-2 px-6 py-16 text-center text-muted">
    <Icon name="box" class="size-10 text-[#C5CBD3]" />
    <p>{{ empty }}</p>
  </div>
  <div v-else-if="asList" class="card px-[18px] py-1">
    <PostListItem v-for="p in posts" :key="p.id" :post="p" />
  </div>
  <div v-else class="grid grid-cols-3 gap-4 max-md:grid-cols-2 max-md:gap-2.5">
    <PostCardItem v-for="p in posts" :key="p.id" :post="p" />
  </div>
</template>
