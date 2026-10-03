<script setup lang="ts">
// 图标来自 public/icons.svg（与设计稿共用一套），浏览器缓存后各页面复用
defineProps<{ name: string }>()
</script>

<template>
  <svg class="icon" aria-hidden="true"><use :href="`/icons.svg#i-${name}`" /></svg>
</template>
