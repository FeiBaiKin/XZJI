<script setup lang="ts">
import type { PostCard } from '~/utils/content'

const api = useApi()
const { tree } = await useCategories()

const { data } = await useAsyncData('home', async () => {
  const [home, latest, articles, hot, tags] = await Promise.all([
    api.GET('/site/home'),
    api.GET('/posts', { params: { query: { type: 'resource', pageSize: 6 } } }),
    api.GET('/posts', { params: { query: { type: 'article', pageSize: 5 } } }),
    api.GET('/posts', { params: { query: { type: 'resource', sort: 'views', pageSize: 5 } } }),
    api.GET('/tags', { params: { query: { limit: 16 } } }),
  ])
  return {
    banners: home.data?.banners ?? [],
    notice: home.data?.notice ?? '',
    latest: latest.data?.items ?? [],
    articles: articles.data?.items ?? [],
    hot: hot.data?.items ?? [],
    tags: tags.data?.items ?? [],
  }
})

// 最新资源的分类页签：只在浏览器端切换，不影响服务端渲染的默认内容
const tab = ref('')
const tabPosts = ref<PostCard[] | null>(null)
const tabLoading = ref(false)
const tabs = computed(() => tree.value.filter(c => c.postCount > 0).slice(0, 4))
async function pickTab(slug: string) {
  if (tab.value === slug) return
  tab.value = slug
  if (!slug) {
    tabPosts.value = null
    return
  }
  tabLoading.value = true
  const { data: res } = await api.GET('/posts', { params: { query: { type: 'resource', category: slug, pageSize: 6 } } })
  if (tab.value === slug) tabPosts.value = res?.items ?? []
  tabLoading.value = false
}
const latest = computed(() => tabPosts.value ?? data.value?.latest ?? [])
const notice = computed(() => (data.value?.notice ?? '').split('\n').map(s => s.trim()).filter(Boolean))
const iconPosts = computed(() => (data.value?.latest ?? []).filter(p => p.resource?.icon))

useSeoMeta({ title: useSiteTitle(undefined) })
</script>

<template>
  <div>
    <section class="grid grid-cols-[minmax(0,1fr)_300px] gap-5 max-[1100px]:grid-cols-1">
      <HomeBanner :banners="data?.banners ?? []" :icons="iconPosts" />
      <UserCard class="max-[1100px]:hidden" />
    </section>

    <section
      v-if="tree.length"
      class="mt-4 grid grid-cols-[repeat(var(--n),minmax(0,1fr))] gap-3.5 max-[1100px]:grid-cols-[repeat(var(--m),minmax(0,1fr))] max-md:gap-2.5"
      :style="{ '--n': Math.min(tree.length, 6), '--m': tree.length === 4 ? 2 : Math.min(tree.length, 3) }"
    >
      <NuxtLink
        v-for="c in tree.slice(0, 6)"
        :key="c.id"
        :to="categoryUrl(c.slug)"
        class="relative flex h-[86px] flex-col justify-center overflow-hidden rounded-card px-[18px] py-4 text-white hover:brightness-105 max-md:h-[72px] max-md:p-3"
        :class="gradientOf(c.color)"
      >
        <b class="text-[17px] font-extrabold tracking-[.02em] max-md:text-[15px]">{{ c.name }}</b>
        <small class="text-[12.5px] opacity-90 max-md:hidden">已收录 {{ compactCount(c.postCount) }} 个</small>
        <Icon :name="c.icon || 'grid'" class="absolute -right-1.5 -bottom-2.5 size-[66px] stroke-[1.6] opacity-30" />
      </NuxtLink>
    </section>

    <div class="mt-5 grid grid-cols-[minmax(0,1fr)_300px] items-start gap-5 max-[1100px]:grid-cols-1">
      <div class="flex flex-col gap-5">
        <section>
          <div class="mb-3 flex items-center gap-4">
            <h2 class="sec-title">最新资源</h2>
            <nav v-if="tabs.length > 1" class="flex gap-1 max-md:hidden" aria-label="按分类查看">
              <button
                v-for="t in [{ slug: '', name: '全部' }, ...tabs]"
                :key="t.slug"
                type="button"
                class="rounded-full px-[11px] py-[3px] text-[13.5px]"
                :class="tab === t.slug ? 'bg-theme-soft font-semibold text-theme' : 'text-muted hover:text-theme'"
                @click="pickTab(t.slug)"
              >
                {{ t.name }}
              </button>
            </nav>
            <NuxtLink v-if="tab" :to="categoryUrl(tab)" class="ml-auto inline-flex items-center gap-0.5 text-[13px] text-muted hover:text-theme">更多<Icon name="right" class="size-3.5" /></NuxtLink>
          </div>
          <PostList :posts="latest" :class="tabLoading && 'opacity-60'" empty="还没有发布资源" />
        </section>

        <section v-if="data?.articles.length">
          <div class="mb-3 flex items-center gap-4">
            <h2 class="sec-title">最新文章</h2>
          </div>
          <PostList :posts="data.articles" />
        </section>
      </div>

      <aside class="flex flex-col gap-5 max-[1100px]:grid max-[1100px]:grid-cols-2 max-md:grid-cols-1">
        <VipCard />
        <SideBox v-if="data?.hot.length" icon="fire" title="热门资源">
          <RankList :posts="data.hot" />
        </SideBox>
        <SideBox v-if="data?.tags.length" icon="tag" title="热门标签">
          <TagCloud :tags="data.tags" />
        </SideBox>
        <SideBox v-if="notice.length" icon="horn" title="本站公告">
          <div class="space-y-1.5 px-4 pb-4 text-[13px] leading-[1.8]">
            <p v-for="(line, i) in notice" :key="i">{{ line }}</p>
          </div>
        </SideBox>
      </aside>
    </div>
  </div>
</template>
