<script setup lang="ts">
const docs = await useSiteDocuments()
const html = computed(() => docs.value?.terms ?? '')

useSeoMeta({
  title: useSiteTitle('用户协议'),
  description: '使用本站前请阅读用户协议：账号、内容版权、付费与退款、免责声明。',
})
// 还没填写时不让搜索引擎收录空白页
if (!html.value) useSeoMeta({ robots: 'noindex' })
</script>

<template>
  <div>
    <CrumbNav :items="[{ label: '用户协议' }]" />
    <LegalDoc class="mt-4" title="用户协议" :html="html" />
  </div>
</template>
