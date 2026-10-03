<script setup lang="ts">
const route = useRoute()
const { user, ready } = useAuth()

const nav = [
  { label: '首页', to: '/' },
  { label: '安卓软件', to: '/category/android' },
  { label: '电脑软件', to: '/category/pc', badge: { text: '热门', class: 'bg-jb-red' } },
  { label: '主题源码', to: '/category/theme' },
  { label: '建站教程', to: '/category/tutorial', badge: { text: '上新', class: 'bg-jb-green' } },
  { label: '会员专区', to: '/vip' },
]
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
          :class="route.path === item.to ? 'text-theme after:absolute after:inset-x-3 after:bottom-0 after:h-[3px] after:rounded-t after:bg-theme' : 'text-body'"
        >
          {{ item.label }}
          <sup v-if="item.badge" class="absolute top-2.5 -right-1.5 rounded rounded-bl-none px-1 py-0.5 text-[10px] leading-none font-semibold text-white" :class="item.badge.class">{{ item.badge.text }}</sup>
        </NuxtLink>
      </nav>
      <div class="ml-auto flex items-center gap-1.5">
        <button class="grid size-9 place-items-center rounded-full text-body hover:bg-soft hover:text-theme" aria-label="搜索">
          <Icon name="search" class="size-[18px]" />
        </button>
        <NuxtLink to="/vip" class="mx-1.5 inline-flex h-8 items-center gap-1.5 rounded-full bg-jb-vip px-3.5 text-[13px] font-semibold text-vip">
          <Icon name="crown" class="size-[15px] text-vip-gold" />开通会员
        </NuxtLink>
        <ClientOnly>
          <NuxtLink v-if="user" to="/user" class="flex items-center gap-2 text-title hover:text-theme">
            <span class="grid size-8 place-items-center rounded-full bg-jb-orange text-sm font-bold text-white">{{ user.nickname.slice(0, 1) }}</span>
            <span class="max-w-[8em] truncate max-md:hidden">{{ user.nickname }}</span>
          </NuxtLink>
          <NuxtLink v-else-if="ready" :to="loginLink" class="text-sm text-body hover:text-theme">登录 / 注册</NuxtLink>
          <template #fallback>
            <span class="w-16" />
          </template>
        </ClientOnly>
      </div>
    </div>
  </header>
</template>
