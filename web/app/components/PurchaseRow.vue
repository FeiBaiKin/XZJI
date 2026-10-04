<script setup lang="ts">
import type { PublicSchemas } from '@xzji/api-client/public'

// 已购内容的一行：图标、标题、购买时间、实付金额和下载入口
defineProps<{ item: PublicSchemas['Purchase'] }>()
</script>

<template>
  <div class="grid grid-cols-[44px_minmax(0,1fr)_auto_auto] items-center gap-3.5 border-b border-line py-3 last:border-b-0 max-md:grid-cols-[44px_minmax(0,1fr)_auto]">
    <AppTile v-if="item.post" :post="item.post" class="size-11 rounded-[11px] text-[44px] shadow-none" />
    <span v-else class="grid size-11 place-items-center rounded-[11px] bg-soft text-muted"><Icon name="box" /></span>
    <span class="min-w-0">
      <b class="block truncate text-[14.5px] leading-[1.4] text-title">{{ item.post?.title ?? item.title }}<em v-if="item.post?.resource?.version" class="ml-1 font-medium text-theme not-italic">v{{ item.post.resource.version }}</em></b>
      <small class="text-xs text-muted">{{ dateTime(item.createdAt) }} 购买{{ item.post ? '' : '，内容已下线' }}</small>
    </span>
    <span class="text-right text-[13px] text-muted max-md:hidden">¥{{ yuan(item.payAmount) }}<template v-if="item.payAmount < item.amount">（会员价）</template></span>
    <NuxtLink v-if="item.post" :to="postUrl(item.postId)" class="btn h-[30px] bg-jb-blue px-3 text-[13px]">{{ item.post.type === 'resource' ? '下载' : '查看' }}</NuxtLink>
    <span v-else />
  </div>
</template>
