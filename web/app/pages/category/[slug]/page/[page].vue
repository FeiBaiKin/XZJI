<script setup lang="ts">
const route = useRoute()
const page = pageParam(route.params.page)
// 第 1 页统一用不带页码的网址
if (page === 1) await navigateTo({ path: categoryUrl(String(route.params.slug)), query: route.query }, { redirectCode: 301 })
</script>

<template>
  <CategoryView :slug="String(route.params.slug)" :page="page" />
</template>
