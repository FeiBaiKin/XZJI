<script setup lang="ts">
const props = defineProps<{ items: { label: string, to?: string }[] }>()
const siteUrl = useRuntimeConfig().public.siteUrl

// 结构化数据的面包屑与页面上的面包屑保持一致：都以首页开头，最后一项不带地址
const crumbs = computed(() => [{ label: '首页', to: '/' }, ...props.items])
useJsonLd(() => ({
  '@context': 'https://schema.org',
  '@type': 'BreadcrumbList',
  itemListElement: crumbs.value.map((it, i) => ({
    '@type': 'ListItem',
    position: i + 1,
    name: it.label,
    ...(it.to && i < crumbs.value.length - 1 ? { item: siteUrl + it.to } : {}),
  })),
}))
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
