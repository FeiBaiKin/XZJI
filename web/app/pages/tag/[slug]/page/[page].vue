<script setup lang="ts">
const route = useRoute()
const page = pageParam(route.params.page)
if (page === 1) await navigateTo(tagUrl(String(route.params.slug)), { redirectCode: 301 })
</script>

<template>
  <TagView :slug="String(route.params.slug)" :page="page" />
</template>
