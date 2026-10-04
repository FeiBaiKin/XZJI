<script setup lang="ts">
import type { PublicSchemas } from '@xzji/api-client/public'

definePageMeta({ middleware: 'auth' })

type Status = PublicSchemas['OrderStatus']
const tabs: { label: string, value?: Status }[] = [
  { label: '全部' }, { label: '待支付', value: 'pending' }, { label: '已支付', value: 'paid' },
  { label: '已关闭', value: 'closed' }, { label: '已退款', value: 'refunded' },
]

const route = useRoute()
const page = computed(() => pageParam(route.query.page))
const status = computed(() => tabs.find(t => t.value === route.query.status)?.value)
const data = ref<PublicSchemas['OrderPage'] | null>(null)

async function load() {
  const { data: res } = await useApi().GET('/me/orders', { params: { query: { page: page.value, pageSize: 20, status: status.value } } })
  data.value = res ?? null
}
watch([page, status], load, { immediate: true })

const payNo = ref('')
const paying = ref(false)
function continuePay(no: string) {
  payNo.value = no
  paying.value = true
}
async function onPaid() {
  await Promise.all([useAuth().refresh(), useAccount().refresh(), load()])
}
</script>

<template>
  <UserShell title="我的订单">
    <section class="card px-5 py-4 max-md:px-3.5">
      <div class="mb-3 flex flex-wrap items-center gap-3">
        <h2 class="sec-title">我的订单</h2>
        <nav class="ml-auto flex gap-1 max-md:ml-0" aria-label="订单状态">
          <NuxtLink
            v-for="t in tabs"
            :key="t.label"
            :to="{ query: t.value ? { status: t.value } : {} }"
            class="rounded-full px-[11px] py-[3px] text-[13.5px]"
            :class="status === t.value ? 'bg-theme-soft font-semibold text-theme' : 'text-muted hover:text-theme'"
          >
            {{ t.label }}
          </NuxtLink>
        </nav>
      </div>
      <template v-if="data">
        <OrderTable v-if="data.items.length" :orders="data.items" @pay="continuePay" />
        <p v-else class="py-12 text-center text-[13px] text-muted">没有{{ status ? tabs.find(t => t.value === status)!.label : '' }}订单</p>
      </template>
      <p v-else class="py-12 text-center text-[13px] text-muted">正在加载…</p>
      <p class="mt-3 text-xs text-muted">待支付订单 30 分钟后自动关闭。付款后如果状态没有变化，刷新页面或联系客服。</p>
    </section>
    <PagerNav v-if="data" :page="data.page" :total="data.total" :page-size="data.pageSize" :to="p => ({ query: { ...route.query, page: p } })" />
    <PayDialog v-model="paying" :order-no="payNo" @paid="onPaid" />
  </UserShell>
</template>
