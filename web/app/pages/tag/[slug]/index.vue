<script setup lang="ts">
const route = useRoute()
</script>

<template>
  <TagView :slug="String(route.params.slug)" :page="1" />
</template>
