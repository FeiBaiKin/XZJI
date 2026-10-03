<script setup lang="ts">
defineProps<{ items: { label: string, to?: string }[] }>()
</script>

<template>
  <nav class="flex flex-wrap items-center gap-1.5 pt-1 pb-3 text-[13px] text-muted" aria-label="当前位置">
    <Icon name="home" class="size-[13px]" />
    <NuxtLink to="/" class="hover:text-theme">首页</NuxtLink>
    <template v-for="(it, i) in items" :key="i">
      <span aria-hidden="true">/</span>
      <NuxtLink v-if="it.to" :to="it.to" class="hover:text-theme">{{ it.label }}</NuxtLink>
      <span v-else>{{ it.label }}</span>
    </template>
  </nav>
</template>
