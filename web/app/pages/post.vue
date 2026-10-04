<script setup lang="ts">
definePageMeta({ path: '/:id(\\d+).html' })

const route = useRoute()
const id = Number(route.params.id)
const api = useApi()
const { byId, parentOf } = await useCategories()

const { data, error } = await useAsyncData(`post:${id}`, async () => {
  const [post, related] = await Promise.all([
    api.GET('/posts/{id}', { params: { path: { id } } }),
    api.GET('/posts/{id}/related', { params: { path: { id }, query: { limit: 3 } } }),
  ])
  notFoundIf(post.response, '内容不存在或已下线')
  const p = post.data!
  const hot = await api.GET('/posts', { params: { query: { category: p.category.slug, sort: 'views', pageSize: 6 } } })
  return {
    post: p,
    related: related.data?.items ?? [],
    hot: (hot.data?.items ?? []).filter(x => x.id !== id).slice(0, 5),
  }
})
throwIfError(error)
const post = computed(() => data.value!.post)
const res = computed(() => post.value.resource)
const category = computed(() => byId(post.value.category.id))
const parent = computed(() => category.value && parentOf(category.value.id))
const top = computed(() => parent.value ?? category.value)
useNavActive(() => top.value && categoryUrl(top.value.slug))

const crumbs = computed(() => [
  ...(parent.value ? [{ label: parent.value.name, to: categoryUrl(parent.value.slug) }] : []),
  { label: post.value.category.name, to: categoryUrl(post.value.category.slug) },
  { label: '正文' },
])

const updated = computed(() => res.value?.changelog[0]?.date || dateOnly(post.value.updatedAt))
const info = computed(() => {
  const r = res.value
  if (!r) return []
  return [
    { name: '资源大小', value: r.fileSize },
    { name: '最近更新', value: updated.value },
    { name: '适用平台', value: r.platform },
    ...r.attrs,
  ].filter(a => a.value)
})

// 文章目录：正文标题加上更新日志
const toc = computed(() => [
  ...post.value.toc,
  ...(res.value?.changelog.length ? [{ id: 'changelog', text: '更新日志', level: 2 }] : []),
])
const activeToc = ref('')
let observer: IntersectionObserver | undefined
onMounted(() => {
  const heads = toc.value.map(t => document.getElementById(t.id)).filter((el): el is HTMLElement => !!el)
  if (!heads.length) return
  const visible = new Set<string>()
  observer = new IntersectionObserver((entries) => {
    for (const e of entries) {
      if (e.isIntersecting) visible.add(e.target.id)
      else visible.delete(e.target.id)
    }
    const first = heads.find(h => visible.has(h.id))
    if (first) activeToc.value = first.id
  }, { rootMargin: '-72px 0px -60% 0px' })
  heads.forEach(h => observer!.observe(h))
})
onBeforeUnmount(() => observer?.disconnect())

// 解锁情况只在浏览器端获取；发表评论后重新获取（不审核时评论可见的内容会立即解锁）
const { state: unlock, refresh: refreshUnlock } = useUnlock(id)
const toast = useToast()
function buy() {
  toast.error('在线支付即将开放，现在可以开通会员免费查看')
}

const image = computed(() => post.value.cover?.url ?? res.value?.icon?.url)
useSeoMeta({
  title: useSiteTitle(() => post.value.seoTitle || post.value.title),
  description: () => post.value.seoDescription || post.value.summary,
  ogType: 'article',
  ogTitle: () => post.value.title,
  ogDescription: () => post.value.seoDescription || post.value.summary,
  ogImage: () => image.value,
})
useHead({ meta: [{ name: 'keywords', content: () => post.value.seoKeywords || undefined }] })
</script>

<template>
  <div v-if="data" class="grid grid-cols-[minmax(0,1fr)_300px] items-start gap-5 max-[1100px]:grid-cols-1">
    <div class="min-w-0">
      <CrumbNav :items="crumbs" />
      <article class="card px-7 pt-6 pb-7 max-md:px-4 max-md:pt-[18px] max-md:pb-5">
        <h1 class="text-[22px] leading-[1.45] font-bold text-title max-md:text-xl">{{ post.title }}</h1>
        <div class="mt-3.5 flex items-center gap-2.5 border-b border-line pb-4">
          <span class="grid size-[38px] place-items-center rounded-full bg-jb-blue text-[15px] font-bold text-white">{{ post.author.nickname.slice(0, 1) }}</span>
          <span>
            <b class="block text-sm leading-[1.4] text-title">{{ post.author.nickname }}</b>
            <small class="block text-xs text-muted"><time :datetime="post.publishedAt">{{ relativeTime(post.publishedAt) }}发布</time></small>
          </span>
          <span class="ml-auto flex gap-3.5 text-[12.5px] text-muted max-md:hidden">
            <span class="inline-flex items-center gap-1"><Icon name="eye" class="size-3.5" />{{ compactCount(post.viewCount) }}</span>
            <span class="inline-flex items-center gap-1"><Icon name="chat" class="size-3.5" />{{ post.commentCount }}</span>
          </span>
        </div>

        <template v-if="res">
          <ResourceBox :post="post" :state="unlock" @buy="buy" />
          <dl v-if="info.length" class="mt-3.5 grid grid-cols-4 overflow-hidden rounded-lg border border-line bg-white max-md:grid-cols-2">
            <!-- 单元格向左上投 1px 阴影作分隔线，最外一圈被容器裁掉 -->
            <div v-for="a in info" :key="a.name" class="px-3.5 py-[9px] text-[13px] shadow-[-1px_-1px_0_var(--color-line)]">
              <dt class="text-xs text-muted">{{ a.name }}</dt>
              <dd class="font-semibold break-all text-title">{{ a.value }}</dd>
            </div>
          </dl>
        </template>

        <RichContent class="mt-4" :html="post.contentHtml" :blocks="post.hiddenBlocks" :state="unlock" :member-free-rank="post.memberFreeRank" @buy="buy" />

        <section v-if="res?.changelog.length" class="mt-2">
          <h2 id="changelog" class="relative mt-[26px] mb-2.5 scroll-mt-20 pl-3.5 text-[19px] leading-[1.4] font-bold text-title before:absolute before:inset-y-[3px] before:left-0 before:w-1 before:rounded before:bg-theme">更新日志</h2>
          <ol class="mt-1.5">
            <li v-for="(l, i) in res.changelog" :key="i" class="relative ml-1.5 border-l-2 border-line pb-3.5 pl-[22px] last:pb-0">
              <span class="absolute top-1.5 -left-[7px] size-3 rounded-full border-2" :class="i === 0 ? 'border-theme bg-theme' : 'border-[#C5CBD3] bg-white'" />
              <b class="text-title">{{ l.version || '更新' }}</b><time v-if="l.date" class="ml-2 text-[12.5px] text-muted">{{ l.date }}</time>
              <p v-if="l.note" class="text-sm leading-[1.8] whitespace-pre-line">{{ l.note }}</p>
            </li>
          </ol>
        </section>

        <div class="mt-[22px] flex flex-wrap gap-2 border-t border-line pt-4">
          <NuxtLink :to="categoryUrl(post.category.slug)" class="rounded bg-theme-soft px-[7px] py-1 text-xs leading-none text-theme">{{ post.category.name }}</NuxtLink>
          <NuxtLink v-for="t in post.tags" :key="t.id" :to="tagUrl(t.slug)" class="rounded bg-soft px-[7px] py-1 text-xs leading-none text-muted hover:text-theme"># {{ t.name }}</NuxtLink>
        </div>
      </article>

      <CommentSection :post-id="post.id" :count="post.commentCount" @changed="refreshUnlock" />

      <section v-if="data.related.length" class="card mt-5 px-[18px] pt-4 pb-[18px]">
        <h2 class="sec-title mb-3">相关推荐</h2>
        <div class="grid grid-cols-3 gap-3.5 max-md:grid-cols-2">
          <PostCardItem v-for="(p, i) in data.related" :key="p.id" :post="p" flat compact :class="i === 2 && 'max-md:hidden'" />
        </div>
      </section>
    </div>

    <aside class="flex flex-col gap-5 pt-[34px] max-[1100px]:pt-0">
      <SideBox v-if="res" icon="box" title="资源信息">
        <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1.5 px-4 pb-4 text-[13px]">
          <template v-if="res.fileSize"><dt class="text-muted">资源大小</dt><dd class="text-right text-title">{{ res.fileSize }}</dd></template>
          <dt class="text-muted">最近更新</dt><dd class="text-right text-title">{{ updated }}</dd>
          <template v-if="res.platform"><dt class="text-muted">适用平台</dt><dd class="text-right text-title">{{ res.platform }}</dd></template>
          <dt class="text-muted">浏览次数</dt><dd class="text-right text-title">{{ post.viewCount.toLocaleString('en-US') }}</dd>
        </dl>
      </SideBox>
      <SideBox v-if="toc.length" icon="list" title="文章目录" class="sticky top-[84px] max-[1100px]:static">
        <ol class="px-4 pb-3.5 text-[13.5px]">
          <li v-for="t in toc" :key="t.id">
            <a
              :href="`#${t.id}`"
              class="block border-l-2 py-1.5 pr-2.5 hover:text-theme"
              :class="[t.level === 3 ? 'pl-6 text-[13px]' : 'pl-2.5', activeToc === t.id ? 'rounded-r-md border-theme bg-theme-soft font-semibold text-theme' : 'border-line']"
            >{{ t.text }}</a>
          </li>
        </ol>
      </SideBox>
      <SideBox v-if="data.hot.length" icon="fire" title="同类热门">
        <RankList :posts="data.hot" />
      </SideBox>
    </aside>
  </div>
</template>
