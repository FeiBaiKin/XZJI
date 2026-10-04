<script setup lang="ts">
import type { LocationQuery } from 'vue-router'

const props = defineProps<{ slug: string, page: number }>()
const pageSize = 18

const route = useRoute()
const api = useApi()
const { bySlug, parentOf } = await useCategories()

const category = computed(() => bySlug(props.slug))
if (!category.value) throw createError({ statusCode: 404, statusMessage: '这个分类不存在', fatal: true })
const parent = computed(() => category.value && parentOf(category.value.id))
// 侧栏和筛选栏以顶级分类为准：在子分类里也能看到同级分类
const top = computed(() => parent.value ?? category.value!)

const platform = computed(() => typeof route.query.platform === 'string' ? route.query.platform : '')
const sort = computed(() => route.query.sort === 'views' ? 'views' : 'latest')
const accessOptions = [
  { value: '', label: '全部' },
  { value: 'free', label: '免费' },
  { value: 'login', label: '登录下载' },
  { value: 'vip', label: '会员免费' },
  { value: 'paid', label: '付费' },
] as const
type Access = 'free' | 'login' | 'vip' | 'paid'
const access = computed(() => accessOptions.find(o => o.value && o.value === route.query.access)?.value as Access | undefined)

const { data, error } = await useAsyncData(
  () => `category:${props.slug}:${props.page}:${platform.value}:${access.value ?? ''}:${sort.value}`,
  async () => {
    const [list, platforms, hot] = await Promise.all([
      api.GET('/posts', { params: { query: { category: props.slug, platform: platform.value || undefined, access: access.value, sort: sort.value, page: props.page, pageSize } } }),
      api.GET('/platforms', { params: { query: { category: props.slug } } }),
      api.GET('/posts', { params: { query: { category: top.value.slug, sort: 'views', pageSize: 5 } } }),
    ])
    notFoundIf(list.response, '这个分类不存在')
    return { list: list.data!, platforms: platforms.data?.items ?? [], hot: hot.data?.items ?? [] }
  },
)
throwIfError(error)
if (props.page > 1 && !data.value?.list.items.length) throw createError({ statusCode: 404, statusMessage: '没有这一页', fatal: true })

// 筛选和排序回到第一页；分页保留筛选条件
const link = (slug: string, query: LocationQuery = {}, page = 1) => ({ path: categoryUrl(slug, page), query })
const keep = (patch: Record<string, string | undefined>): LocationQuery => {
  const q = { platform: platform.value || undefined, access: access.value, sort: sort.value === 'views' ? 'views' : undefined, ...patch }
  return Object.fromEntries(Object.entries(q).filter((e): e is [string, string] => !!e[1]))
}

const title = computed(() => {
  const base = category.value!.seoTitle || category.value!.name
  return props.page > 1 ? `${base} - 第 ${props.page} 页` : base
})
useSeoMeta({
  title: useSiteTitle(title),
  description: () => category.value!.seoDescription || category.value!.description || undefined,
})
</script>

<template>
  <div v-if="category && data">
    <CrumbNav :items="parent ? [{ label: parent.name, to: categoryUrl(parent.slug) }, { label: category.name }] : [{ label: category.name }]" />

    <section class="card relative flex items-center gap-5 overflow-hidden px-6 py-[22px] after:absolute after:-top-20 after:-right-[60px] after:size-[260px] after:rounded-full after:bg-[radial-gradient(circle,var(--color-theme-soft)_0_60%,transparent_61%)] max-md:px-4">
      <span class="relative z-10 grid size-16 shrink-0 place-items-center rounded-2xl text-white max-md:size-12" :class="gradientOf(category.color || top.color)">
        <Icon :name="category.icon || top.icon || 'grid'" class="size-8 max-md:size-6" />
      </span>
      <div class="relative z-10 min-w-0">
        <h1 class="text-[22px] leading-[1.3] font-bold text-title max-md:text-lg">{{ category.name }}</h1>
        <p v-if="category.description" class="mt-0.5 text-muted">{{ category.description }}</p>
        <div class="mt-2 text-[13px] text-muted">共<b class="mx-0.5 text-[15px] text-title">{{ category.postCount.toLocaleString('en-US') }}</b>个</div>
      </div>
    </section>

    <section v-if="top.children.length || data.platforms.length" class="card mt-4 px-5 py-1 max-md:px-4" aria-label="筛选">
      <div v-if="top.children.length" class="flex items-start gap-4 py-2.5 [&+&]:border-t [&+&]:border-dashed [&+&]:border-line">
        <span class="w-10 shrink-0 text-[13.5px] leading-7 text-muted">分类</span>
        <div class="flex flex-wrap gap-1.5">
          <NuxtLink
            v-for="c in [top, ...top.children]"
            :key="c.id"
            :to="link(c.slug, keep({ platform: undefined }))"
            class="rounded-[5px] px-3 text-[13.5px] leading-7"
            :class="c.id === category.id ? 'bg-theme text-white' : 'hover:text-theme'"
          >
            {{ c.id === top.id ? '全部' : c.name }}<i v-if="c.id !== top.id" class="ml-[3px] text-xs not-italic" :class="c.id === category.id ? 'text-white/75' : 'text-muted'">{{ c.postCount }}</i>
          </NuxtLink>
        </div>
      </div>
      <div v-if="data.platforms.length" class="flex items-start gap-4 py-2.5 [&+&]:border-t [&+&]:border-dashed [&+&]:border-line">
        <span class="w-10 shrink-0 text-[13.5px] leading-7 text-muted">平台</span>
        <div class="flex flex-wrap gap-1.5">
          <NuxtLink
            v-for="p in [{ name: '', count: 0 }, ...data.platforms]"
            :key="p.name"
            :to="link(slug, keep({ platform: p.name || undefined }))"
            class="rounded-[5px] px-3 text-[13.5px] leading-7"
            :class="platform === p.name ? 'bg-theme text-white' : 'hover:text-theme'"
          >
            {{ p.name || '全部' }}<i v-if="p.name" class="ml-[3px] text-xs not-italic" :class="platform === p.name ? 'text-white/75' : 'text-muted'">{{ p.count }}</i>
          </NuxtLink>
        </div>
      </div>
      <div v-if="data.platforms.length || access" class="flex items-start gap-4 py-2.5 [&+&]:border-t [&+&]:border-dashed [&+&]:border-line">
        <span class="w-10 shrink-0 text-[13.5px] leading-7 text-muted">价格</span>
        <div class="flex flex-wrap gap-1.5">
          <NuxtLink
            v-for="o in accessOptions"
            :key="o.value"
            :to="link(slug, keep({ access: o.value || undefined }))"
            class="rounded-[5px] px-3 text-[13.5px] leading-7"
            :class="(access ?? '') === o.value ? 'bg-theme text-white' : 'hover:text-theme'"
          >
            {{ o.label }}
          </NuxtLink>
        </div>
      </div>
    </section>

    <div class="mt-5 grid grid-cols-[minmax(0,1fr)_300px] items-start gap-5 max-[1100px]:grid-cols-1">
      <div>
        <div class="mb-3 flex items-center gap-4 text-[13.5px] text-muted">
          <span>找到<b class="mx-0.5 text-title">{{ data.list.total.toLocaleString('en-US') }}</b>个</span>
          <nav class="ml-auto flex gap-1" aria-label="排序">
            <NuxtLink :to="link(slug, keep({ sort: undefined }))" class="rounded-[5px] px-2.5 py-[3px]" :class="sort === 'latest' ? 'bg-theme-soft font-semibold text-theme' : 'hover:text-theme'">最新发布</NuxtLink>
            <NuxtLink :to="link(slug, keep({ sort: 'views' }))" class="rounded-[5px] px-2.5 py-[3px]" :class="sort === 'views' ? 'bg-theme-soft font-semibold text-theme' : 'hover:text-theme'">最多浏览</NuxtLink>
          </nav>
        </div>
        <PostList :posts="data.list.items" :empty="platform || access ? '没有符合筛选条件的内容，换个条件试试' : '这个分类下还没有内容'" />
        <PagerNav :page="page" :total="data.list.total" :page-size="pageSize" :to="p => link(slug, keep({}), p)" />
      </div>

      <aside class="flex flex-col gap-5 max-[1100px]:grid max-[1100px]:grid-cols-2 max-md:grid-cols-1">
        <SideBox v-if="top.children.length" icon="list" :title="`${top.name}分类`">
          <ul class="px-2.5 pb-3">
            <li v-for="c in [top, ...top.children]" :key="c.id">
              <NuxtLink
                :to="categoryUrl(c.slug)"
                class="flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm"
                :class="c.id === category.id ? 'bg-theme-soft font-semibold text-theme' : 'hover:bg-soft hover:text-theme'"
              >
                <Icon :name="c.id === top.id ? 'grid' : (c.icon || 'list')" :class="c.id === category.id ? 'text-theme' : 'text-muted'" />
                {{ c.id === top.id ? '全部' : c.name }}
                <i class="ml-auto text-xs not-italic text-muted">{{ c.postCount.toLocaleString('en-US') }}</i>
              </NuxtLink>
            </li>
          </ul>
        </SideBox>
        <SideBox v-if="data.hot.length" icon="fire" :title="`${top.name}热门`">
          <RankList :posts="data.hot" />
        </SideBox>
        <VipCard />
      </aside>
    </div>
  </div>
</template>
