<script setup lang="ts">
// 用户协议、隐私政策两个页面的共用版式；cssClass 只用来调段落字号
defineProps<{ title: string, html: string }>()
</script>

<template>
  <section class="card px-8 py-7 max-md:px-4">
    <h1 class="text-[22px] font-bold text-title max-md:text-lg">{{ title }}</h1>
    <!-- 站长未填写时提示，页面同时输出 noindex -->
    <p v-if="!html" class="mt-4 text-[14px] text-muted">站长尚未填写{{ title }}。</p>
    <!-- 正文已在后端按内容正文的同一套白名单过滤 -->
    <div v-else class="prose-x mt-4 text-[15px]" v-html="html" />
  </section>
</template>
