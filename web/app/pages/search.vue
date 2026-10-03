<script setup lang="ts">
const route = useRoute()
const api = useApi()
const pageSize = 18
await useCategories()

const q = computed(() => (typeof route.query.q === 'string' ? route.query.q : '').trim().slice(0, 50))
const page = computed(() => pageParam(route.query.page))
const input = ref(q.value)
watch(q, (v) => { input.value = v })

const { data } = await useAsyncData(() => `search:${q.value}:${page.value}`, async () => {
  if (!q.value) return null
  const { data } = await api.GET('/posts', { params: { query: { q: q.value, page: page.value, pageSize } } })
  return data ?? null
})

function submit() {
  const v = input.value.trim()
  navigateTo({ path: '/search', query: v ? { q: v } : {} })
}

useSeoMeta({
  title: useSiteTitle(() => q.value ? `搜索：${q.value}` : '搜索'),
  robots: 'noindex, follow',
})
</script>

<template>
  <div>
    <CrumbNav :items="[{ label: '搜索' }]" />
    <section class="card px-6 py-6 max-md:px-4">
      <h1 class="text-[22px] font-bold text-title max-md:text-lg">搜索资源和文章</h1>
      <form class="mt-3 flex gap-2.5" role="search" @submit.prevent="submit">
        <input v-model="input" type="search" maxlength="50" placeholder="输入软件名、主题名或教程关键词" aria-label="搜索关键词" class="field-input flex-1">
        <button type="submit" class="btn h-[42px] bg-jb-blue px-6"><Icon name="search" />搜索</button>
      </form>
      <p v-if="data" class="mt-3 text-[13.5px] text-muted">“{{ q }}”找到<b class="mx-0.5 text-title">{{ data.total.toLocaleString('en-US') }}</b>个结果</p>
    </section>
    <div v-if="data" class="mt-5">
      <PostList :posts="data.items" :empty="`没有找到和“${q}”相关的内容，换个关键词试试`" />
      <PagerNav :page="page" :total="data.total" :page-size="pageSize" :to="p => ({ path: '/search', query: p > 1 ? { q, page: p } : { q } })" />
    </div>
  </div>
</template>
