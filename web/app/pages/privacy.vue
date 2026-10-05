<script setup lang="ts">
const docs = await useSiteDocuments()
const html = computed(() => docs.value?.privacy ?? '')

useSeoMeta({
  title: useSiteTitle('隐私政策'),
  description: '本站收集哪些信息、用在什么地方、保存多久，以及你可以怎样查询和删除。',
})
// 还没填写时不让搜索引擎收录空白页
if (!html.value) useSeoMeta({ robots: 'noindex' })
</script>

<template>
  <div>
    <CrumbNav :items="[{ label: '隐私政策' }]" />
    <LegalDoc class="mt-4" title="隐私政策" :html="html" />
  </div>
</template>
