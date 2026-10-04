<script setup lang="ts">
import type { PublicSchemas } from '@xzji/api-client/public'

definePageMeta({ middleware: 'auth' })

const route = useRoute()
const page = computed(() => pageParam(route.query.page))
const data = ref<PublicSchemas['PurchasePage'] | null>(null)

watch(page, async (p) => {
  const { data: res } = await useApi().GET('/me/purchases', { params: { query: { page: p, pageSize: 20 } } })
  data.value = res ?? null
}, { immediate: true })
</script>

<template>
  <UserShell title="已购资源">
    <section class="card px-5 py-4 max-md:px-3.5">
      <h2 class="sec-title mb-1">已购资源</h2>
      <p class="mb-1 text-[12.5px] text-muted">购买后永久有效，资源更新后也可以直接下载新版本</p>
      <template v-if="data">
        <PurchaseRow v-for="p in data.items" :key="p.postId" :item="p" />
        <p v-if="!data.items.length" class="py-12 text-center text-[13px] text-muted">还没有购买过内容，<NuxtLink to="/" class="text-theme">去首页看看</NuxtLink></p>
      </template>
      <p v-else class="py-12 text-center text-[13px] text-muted">正在加载…</p>
    </section>
    <PagerNav v-if="data" :page="data.page" :total="data.total" :page-size="data.pageSize" :to="p => ({ query: { page: p } })" />
  </UserShell>
</template>
