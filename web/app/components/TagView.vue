<script setup lang="ts">
const props = defineProps<{ slug: string, page: number }>()
const pageSize = 18
const api = useApi()
await useCategories()

const { data, error } = await useAsyncData(`tag:${props.slug}:${props.page}`, async () => {
  const [tag, list, hot] = await Promise.all([
    api.GET('/tags/{slug}', { params: { path: { slug: props.slug } } }),
    api.GET('/posts', { params: { query: { tag: props.slug, page: props.page, pageSize } } }),
    api.GET('/tags', { params: { query: { limit: 20 } } }),
  ])
  notFoundIf(tag.response, '这个标签不存在')
  return { tag: tag.data!, list: list.data!, hot: hot.data?.items ?? [] }
})
throwIfError(error)
if (props.page > 1 && !data.value?.list.items.length) throw createError({ statusCode: 404, statusMessage: '没有这一页', fatal: true })

const title = computed(() => data.value ? `${data.value.tag.name}${props.page > 1 ? ` - 第 ${props.page} 页` : ''}` : '')
useSeoMeta({ title: useSiteTitle(title), description: () => data.value ? `标签“${data.value.tag.name}”下的全部内容` : undefined })
</script>

<template>
  <div v-if="data">
    <CrumbNav :items="[{ label: '标签' }, { label: data.tag.name }]" />
    <section class="card flex items-center gap-5 px-6 py-[22px] max-md:px-4">
      <span class="grid size-16 shrink-0 place-items-center rounded-2xl bg-jb-blue text-white max-md:size-12"><Icon name="tag" class="size-8 max-md:size-6" /></span>
      <div>
        <h1 class="text-[22px] leading-[1.3] font-bold text-title max-md:text-lg"># {{ data.tag.name }}</h1>
        <div class="mt-1 text-[13px] text-muted">共<b class="mx-0.5 text-[15px] text-title">{{ data.tag.postCount.toLocaleString('en-US') }}</b>个</div>
      </div>
    </section>
    <div class="mt-5 grid grid-cols-[minmax(0,1fr)_300px] items-start gap-5 max-[1100px]:grid-cols-1">
      <div>
        <PostList :posts="data.list.items" empty="这个标签下还没有内容" />
        <PagerNav :page="page" :total="data.list.total" :page-size="pageSize" :to="p => tagUrl(slug, p)" />
      </div>
      <aside class="flex flex-col gap-5">
        <SideBox v-if="data.hot.length" icon="tag" title="热门标签">
          <TagCloud :tags="data.hot" />
        </SideBox>
        <VipCard />
      </aside>
    </div>
  </div>
</template>
