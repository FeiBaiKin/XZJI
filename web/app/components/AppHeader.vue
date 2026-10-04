<script setup lang="ts">
const route = useRoute()
const { user, ready } = useAuth()
const [{ tree }] = await Promise.all([useCategories(), useLevels()])

// 导航：首页 + 顶级分类 + 会员专区。当前页属于某个顶级分类（含子分类）时，该项高亮
const nav = computed(() => [
  { label: '首页', to: '/', match: (p: string) => p === '/' },
  ...tree.value.map(c => ({
    label: c.name,
    to: categoryUrl(c.slug),
    match: (p: string) => [c, ...c.children].some(x => p.startsWith(`/category/${x.slug}/`) || p === `/category/${x.slug}`),
  })),
  { label: '会员专区', to: '/vip', match: (p: string) => p.startsWith('/vip') },
])
// 详情页的网址不含分类，由详情页通过 useNavActive 指定高亮项，只对当前路径有效。
// 页头可能先于页面渲染，挂载后才采用这个指定，避免服务端与浏览器渲染结果不一致
const navActive = useState<{ path: string, to: string } | null>('nav:active', () => null)
const mounted = ref(false)
onMounted(() => { mounted.value = true })
const isActive = (item: { to: string, match: (p: string) => boolean }) =>
  mounted.value && navActive.value?.path === route.path ? navActive.value.to === item.to : item.match(route.path)
const loginLink = computed(() => ({ path: '/login', query: route.path === '/' ? {} : { redirect: route.fullPath } }))
</script>

<template>
  <header class="sticky top-0 z-30 bg-white/95 shadow-[0_1px_0_var(--color-line)] backdrop-blur">
    <div class="mx-auto flex h-16 max-w-[1200px] items-center gap-7 px-5 max-md:h-14 max-md:gap-3 max-md:px-3">
      <AppLogo />
      <nav class="flex h-full gap-1 max-[1100px]:hidden" aria-label="主导航">
        <NuxtLink
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          class="relative flex items-center px-3 text-[15px] hover:text-theme"
          :class="isActive(item) ? 'text-theme after:absolute after:inset-x-3 after:bottom-0 after:h-[3px] after:rounded-t after:bg-theme' : 'text-body'"
        >
          {{ item.label }}
        </NuxtLink>
      </nav>
      <div class="ml-auto flex items-center gap-1.5">
        <NuxtLink to="/search" class="grid size-9 place-items-center rounded-full text-body hover:bg-soft hover:text-theme" aria-label="搜索">
          <Icon name="search" class="size-[18px]" />
        </NuxtLink>
        <NuxtLink to="/vip" class="mx-1.5 inline-flex h-8 items-center gap-1.5 rounded-full bg-jb-vip px-3.5 text-[13px] font-semibold text-vip">
          <Icon name="crown" class="size-[15px] text-vip-gold" />开通会员
        </NuxtLink>
        <ClientOnly>
          <NuxtLink v-if="user" to="/user" class="flex items-center gap-2 text-title hover:text-theme">
            <span class="relative grid size-8 place-items-center rounded-full bg-jb-orange text-sm font-bold text-white">
              {{ user.nickname.slice(0, 1) }}
              <Icon v-if="user.membership" name="crown" class="absolute -right-1 -bottom-[3px] size-[15px] rounded-full bg-[#2A2A31] p-0.5 text-vip-gold" :title="user.membership.name" />
            </span>
            <span class="max-w-[8em] truncate max-md:hidden">{{ user.nickname }}</span>
          </NuxtLink>
          <NuxtLink v-else-if="ready" :to="loginLink" class="text-sm text-body hover:text-theme max-md:hidden">登录 / 注册</NuxtLink>
          <template #fallback>
            <span class="w-16" />
          </template>
        </ClientOnly>
      </div>
    </div>
  </header>
</template>
