<script setup lang="ts">
// 首页右侧的用户卡片：登录状态只在浏览器端获取
const { user, ready } = useAuth()
const siteName = useRuntimeConfig().public.siteName
const perks = [
  { icon: 'crown', label: '会员免费', class: 'bg-jb-vip text-vip-gold' },
  { icon: 'refresh', label: '失效补链', class: 'bg-jb-green text-white' },
  { icon: 'box', label: '解压即用', class: 'bg-jb-blue text-white' },
]
</script>

<template>
  <aside class="card overflow-hidden pb-5 text-center">
    <div class="h-[86px] bg-[radial-gradient(circle_at_20%_120%,rgba(255,255,255,.35)_0_30%,transparent_31%),linear-gradient(120deg,#9CC2FF,#C4B5FF)]" />
    <ClientOnly>
      <template v-if="user">
        <div class="mx-auto -mt-[34px] grid size-[68px] place-items-center rounded-full border-3 border-white bg-jb-orange text-[26px] font-bold text-white">{{ user.nickname.slice(0, 1) }}</div>
        <h3 class="mt-2 text-base font-bold text-title">Hi，{{ user.nickname }}</h3>
        <p class="mx-[22px] mt-1 text-[13px] text-muted">欢迎回来，个人中心可以修改资料和密码</p>
        <div class="mx-[22px] mt-4 grid grid-cols-2 gap-2.5">
          <NuxtLink to="/user" class="btn bg-jb-blue">个人中心</NuxtLink>
          <NuxtLink to="/vip" class="btn bg-jb-vip text-vip">开通会员</NuxtLink>
        </div>
      </template>
      <template v-else>
        <div class="mx-auto -mt-[34px] grid size-[68px] place-items-center rounded-full border-3 border-white bg-soft text-[#B7BDC6]"><Icon name="user" class="size-8" /></div>
        <h3 class="mt-2 text-base font-bold text-title">Hi，欢迎来到 {{ siteName }}</h3>
        <p class="mx-[22px] mt-1 text-[13px] text-muted">登录后可以查看已购资源和下载记录</p>
        <div class="mx-[22px] mt-4 grid grid-cols-2 gap-2.5" :class="!ready && 'invisible'">
          <NuxtLink to="/login" class="btn bg-jb-blue">登录</NuxtLink>
          <NuxtLink to="/login?tab=register" class="btn bg-jb-orange">注册</NuxtLink>
        </div>
      </template>
      <template #fallback>
        <div class="mx-auto -mt-[34px] size-[68px] rounded-full border-3 border-white bg-soft" />
        <div class="h-[106px]" />
      </template>
    </ClientOnly>
    <div class="mx-4 mt-[18px] grid grid-cols-3 border-t border-dashed border-line pt-3.5">
      <span v-for="p in perks" :key="p.label" class="flex flex-col items-center gap-1 text-[12.5px]">
        <b class="grid size-[34px] place-items-center rounded-[10px]" :class="p.class"><Icon :name="p.icon" class="size-[17px]" /></b>{{ p.label }}
      </span>
    </div>
  </aside>
</template>
