<script setup lang="ts">
// 用户中心外壳：顶部资料卡和左侧菜单，各子页面放在默认插槽里
const props = defineProps<{ title: string }>()
useSeoMeta({ title: useSiteTitle(() => props.title), robots: 'noindex' })

const auth = useAuth()
const user = computed(() => auth.user.value!)
const { overview, refresh } = useAccount()
onMounted(refresh)

const route = useRoute()
const menu = computed(() => [
  { to: '/user', icon: 'home', label: '账户概览' },
  { to: '/user/purchases', icon: 'box', label: '已购资源', count: overview.value?.purchaseCount },
  { to: '/user/orders', icon: 'list', label: '我的订单' },
  { to: '/user/wallet', icon: 'wallet', label: '我的钱包' },
  { to: '/vip', icon: 'crown', label: '会员中心' },
  { to: '/user/settings', icon: 'user', label: '账号设置' },
])

// 邮箱只露出前三位，截图时不泄露完整地址
const maskedEmail = computed(() => {
  const [name = '', domain = ''] = user.value.email.split('@')
  return `${name.slice(0, 3)}***@${domain}`
})

async function logout() {
  await auth.logout()
  await navigateTo('/', { replace: true })
}
</script>

<template>
  <div class="pb-2">
    <section class="card overflow-hidden">
      <div class="h-[120px] bg-brand" />
      <div class="-mt-[34px] flex flex-wrap items-end gap-4 px-6 pb-[18px] max-md:px-4">
        <span class="grid size-[84px] shrink-0 place-items-center rounded-full border-4 border-white bg-jb-orange text-[32px] font-bold text-white shadow-card">{{ user.nickname.slice(0, 1) }}</span>
        <div class="min-w-0">
          <h1 class="flex items-center gap-2 truncate text-xl leading-[1.4] font-bold text-title">
            {{ user.nickname }}
            <span v-if="user.membership" class="inline-flex items-center gap-[3px] rounded-full bg-jb-vip px-2 py-1 text-xs leading-none font-bold text-[#F6D48F]">
              <Icon name="crown" class="size-3" />{{ user.membership.name }}
            </span>
          </h1>
          <p class="flex flex-wrap gap-x-4 gap-y-1 text-[12.5px] text-muted">
            <span>ID {{ user.id }}</span><span>注册于 {{ dateOnly(user.createdAt) }}</span><span>邮箱 {{ maskedEmail }}</span>
          </p>
        </div>
        <div class="ml-auto flex gap-2.5 max-md:ml-0">
          <NuxtLink to="/user/settings" class="btn btn-soft">编辑资料</NuxtLink>
          <NuxtLink to="/vip" class="btn bg-jb-gold text-[#5B3A00]">{{ user.membership ? '续费或升级' : '开通会员' }}</NuxtLink>
        </div>
      </div>
    </section>

    <div class="mt-5 grid grid-cols-[200px_minmax(0,1fr)] items-start gap-5 max-md:grid-cols-1">
      <nav class="card sticky top-[84px] p-2.5 max-md:static max-md:flex max-md:gap-1 max-md:overflow-x-auto" aria-label="用户中心菜单">
        <NuxtLink
          v-for="m in menu"
          :key="m.to"
          :to="m.to"
          class="flex items-center gap-2.5 rounded-md px-3 py-2.5 text-sm whitespace-nowrap max-md:py-2"
          :class="route.path === m.to ? 'bg-theme-soft font-semibold text-theme' : 'hover:bg-soft hover:text-theme'"
        >
          <Icon :name="m.icon" />{{ m.label }}
          <i v-if="m.count" class="ml-auto rounded-full px-[7px] text-xs leading-[18px] not-italic max-md:hidden" :class="route.path === m.to ? 'bg-white text-theme' : 'bg-soft text-muted'">{{ m.count }}</i>
        </NuxtLink>
        <hr class="my-1.5 border-line max-md:hidden">
        <button type="button" class="flex w-full items-center gap-2.5 rounded-md px-3 py-2.5 text-sm whitespace-nowrap hover:bg-soft hover:text-theme max-md:w-auto max-md:py-2" @click="logout">
          <Icon name="logout" />退出登录
        </button>
      </nav>
      <div class="min-w-0">
        <slot />
      </div>
    </div>
  </div>
</template>
