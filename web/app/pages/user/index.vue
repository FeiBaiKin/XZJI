<script setup lang="ts">
import type { PublicSchemas } from '@xzji/api-client/public'
import type { Order } from '~/utils/order'

definePageMeta({ middleware: 'auth' })

const api = useApi()
const auth = useAuth()
const user = computed(() => auth.user.value!)
const { overview, refresh } = useAccount()

const purchases = ref<PublicSchemas['Purchase'][]>([])
const orders = ref<Order[]>([])
const ledger = ref<PublicSchemas['LedgerEntry'][]>([])

async function load() {
  const [p, o, l] = await Promise.all([
    api.GET('/me/purchases', { params: { query: { pageSize: 3 } } }),
    api.GET('/me/orders', { params: { query: { pageSize: 5 } } }),
    api.GET('/me/wallet/ledger', { params: { query: { pageSize: 5 } } }),
  ])
  purchases.value = p.data?.items ?? []
  orders.value = o.data?.items ?? []
  ledger.value = l.data?.items ?? []
}
onMounted(load)

const vip = computed(() => user.value.membership)
const usedPct = computed(() => {
  const o = overview.value
  return o && o.dailyLimit > 0 ? Math.min(100, Math.round(o.todayFreeDownloads / o.dailyLimit * 100)) : 0
})

const payNo = ref('')
const paying = ref(false)
function continuePay(no: string) {
  payNo.value = no
  paying.value = true
}
async function onPaid() {
  await Promise.all([auth.refresh(), refresh(), load()])
}
</script>

<template>
  <UserShell title="用户中心">
    <div class="grid grid-cols-4 gap-4 max-[1100px]:grid-cols-2 max-md:gap-3">
      <div class="card relative flex flex-col px-[18px] py-4">
        <span class="text-[13px] text-muted">账户余额</span>
        <b class="text-[22px] leading-normal text-title">¥{{ yuan(overview?.balance ?? 0) }}</b>
        <small class="text-xs text-muted">只能用于站内消费</small>
        <NuxtLink to="/user/wallet" class="btn absolute top-3.5 right-3.5 h-[26px] bg-jb-blue px-2.5 text-xs">充值</NuxtLink>
      </div>
      <div class="relative flex flex-col rounded-card bg-jb-vip px-[18px] py-4 shadow-card">
        <span class="text-[13px] text-vip/70">{{ vip ? `${vip.name} 会员` : '会员' }}</span>
        <b class="text-[17px] leading-[1.9] whitespace-nowrap text-[#F6D48F] max-md:text-[15px]">{{ vip ? (vip.expiresAt ? `${dateOnly(vip.expiresAt)} 到期` : '永久有效') : '还没有开通' }}</b>
        <small class="text-xs text-vip/70">{{ vip?.expiresAt ? `还剩 ${daysLeft(vip.expiresAt)} 天` : vip ? '无需续费' : '会员资源免费下载' }}</small>
        <NuxtLink to="/vip" class="btn absolute top-3.5 right-3.5 h-[26px] bg-jb-gold px-2.5 text-xs text-[#5B3A00]">{{ vip ? '续费' : '开通' }}</NuxtLink>
      </div>
      <div class="card flex flex-col px-[18px] py-4">
        <span class="text-[13px] text-muted">今天免费下载</span>
        <template v-if="vip && overview">
          <b class="text-[22px] leading-normal text-title">{{ overview.todayFreeDownloads }}<template v-if="overview.dailyLimit"> / {{ overview.dailyLimit }}</template></b>
          <span v-if="overview.dailyLimit" class="my-1.5 h-1.5 overflow-hidden rounded-full bg-soft"><i class="block h-full rounded-full bg-jb-green" :style="{ width: `${usedPct}%` }" /></span>
          <small class="text-xs text-muted">{{ overview.dailyLimit ? '明天 0 点重置' : '不限次数' }}</small>
        </template>
        <template v-else>
          <b class="text-[22px] leading-normal text-title">—</b>
          <small class="text-xs text-muted">开通会员后可以免费下载</small>
        </template>
      </div>
      <div class="card flex flex-col px-[18px] py-4">
        <span class="text-[13px] text-muted">已购资源</span>
        <b class="text-[22px] leading-normal text-title">{{ overview?.purchaseCount ?? 0 }}</b>
        <small class="text-xs text-muted">累计消费 ¥{{ yuan(overview?.spent ?? 0) }}</small>
      </div>
    </div>

    <section class="card mt-5 px-5 py-4 max-md:px-3.5">
      <div class="mb-1 flex items-center">
        <h2 class="sec-title">最近购买</h2>
        <NuxtLink to="/user/purchases" class="ml-auto inline-flex items-center text-[13px] text-muted hover:text-theme">全部已购<Icon name="right" class="size-3.5" /></NuxtLink>
      </div>
      <PurchaseRow v-for="p in purchases" :key="p.postId" :item="p" />
      <p v-if="!purchases.length" class="py-8 text-center text-[13px] text-muted">还没有购买过内容</p>
    </section>

    <section class="card mt-5 px-5 py-4 max-md:px-3.5">
      <div class="mb-3 flex items-center">
        <h2 class="sec-title">我的订单</h2>
        <NuxtLink to="/user/orders" class="ml-auto inline-flex items-center text-[13px] text-muted hover:text-theme">全部订单<Icon name="right" class="size-3.5" /></NuxtLink>
      </div>
      <OrderTable v-if="orders.length" :orders="orders" @pay="continuePay" />
      <p v-else class="py-8 text-center text-[13px] text-muted">还没有订单</p>
    </section>

    <section class="card mt-5 px-5 py-4 max-md:px-3.5">
      <div class="mb-3 flex items-center">
        <h2 class="sec-title">余额明细</h2>
        <NuxtLink to="/user/wallet" class="ml-auto inline-flex items-center text-[13px] text-muted hover:text-theme">全部明细<Icon name="right" class="size-3.5" /></NuxtLink>
      </div>
      <LedgerTable v-if="ledger.length" :entries="ledger" />
      <p v-else class="py-8 text-center text-[13px] text-muted">还没有余额变动</p>
    </section>

    <PayDialog v-model="paying" :order-no="payNo" @paid="onPaid" />
  </UserShell>
</template>
