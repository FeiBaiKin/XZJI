<script setup lang="ts">
// canonical 与 og:url 统一指向当前路径的绝对地址，不带查询参数（?ref= 推广参数不能进 canonical）；
// 分页用的是路径，所以指向自身即可
const { public: { siteUrl } } = useRuntimeConfig()
const route = useRoute()
const url = computed(() => siteUrl + route.path)
useHead({ link: [{ rel: 'canonical', href: url }] })
useSeoMeta({ ogUrl: url })
</script>

<template>
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
</template>
