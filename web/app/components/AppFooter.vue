<script setup lang="ts">
const { tree } = await useCategories()
const info = await useSiteInfo()
const siteName = useRuntimeConfig().public.siteName
</script>

<template>
  <footer class="mt-10 bg-card pt-8 pb-10 text-[13px] text-muted shadow-[0_-1px_0_var(--color-line)]">
    <div class="mx-auto grid max-w-[1200px] grid-cols-[1.4fr_repeat(3,1fr)] gap-6 px-5 max-[1100px]:grid-cols-3 max-md:grid-cols-2 max-md:px-3">
      <div class="max-[1100px]:col-span-full">
        <h4 class="mb-2 text-sm font-semibold text-title">{{ siteName }}</h4>
        <p>{{ info?.footerAbout }}</p>
      </div>
      <div>
        <h4 class="mb-2 text-sm font-semibold text-title">资源分类</h4>
        <nav class="flex flex-col gap-1">
          <NuxtLink v-for="c in tree" :key="c.id" :to="categoryUrl(c.slug)" class="hover:text-theme">{{ c.name }}</NuxtLink>
        </nav>
      </div>
      <div>
        <h4 class="mb-2 text-sm font-semibold text-title">帮助</h4>
        <nav class="flex flex-col gap-1">
          <NuxtLink to="/vip" class="hover:text-theme">会员说明</NuxtLink>
          <NuxtLink to="/search" class="hover:text-theme">搜索资源</NuxtLink>
          <NuxtLink to="/terms" class="hover:text-theme">用户协议</NuxtLink>
          <NuxtLink to="/privacy" class="hover:text-theme">隐私政策</NuxtLink>
        </nav>
      </div>
      <div>
        <h4 class="mb-2 text-sm font-semibold text-title">账号</h4>
        <nav class="flex flex-col gap-1">
          <NuxtLink to="/login" class="hover:text-theme">登录</NuxtLink>
          <NuxtLink to="/login?tab=register" class="hover:text-theme">注册</NuxtLink>
          <NuxtLink to="/user" class="hover:text-theme">个人中心</NuxtLink>
        </nav>
      </div>
      <div class="col-span-full flex flex-wrap items-center justify-between gap-x-4 gap-y-2 border-t border-line pt-4">
        <span>© {{ new Date().getFullYear() }} {{ siteName }}</span>
        <span v-if="info?.icp || info?.policeIcp" class="flex flex-wrap items-center gap-x-4 gap-y-1">
          <a v-if="info?.icp" :href="icpUrl" target="_blank" rel="noopener nofollow" class="hover:text-theme">{{ info.icp }}</a>
          <a v-if="info?.policeIcp" :href="policeUrl(info.policeIcp)" target="_blank" rel="noopener nofollow" class="hover:text-theme">{{ info.policeIcp }}</a>
        </span>
      </div>
    </div>
  </footer>
</template>
