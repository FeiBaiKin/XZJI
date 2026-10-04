<script setup lang="ts">
import { errorMessage } from '@xzji/api-client/public'
import type { Order } from '~/utils/order'

// 从支付宝、微信付款页面跳回后确认结果：轮询订单状态，后端每次会顺便向渠道查询
definePageMeta({ middleware: 'auth' })
useSeoMeta({ title: useSiteTitle('支付结果'), robots: 'noindex' })

const route = useRoute()
const no = computed(() => typeof route.query.no === 'string' ? route.query.no : '')
const order = ref<Order | null>(null)
const error = ref('')
const waiting = ref(true)
let timer: ReturnType<typeof setTimeout> | undefined
let tries = 0

async function check() {
  if (!no.value) {
    error.value = '缺少订单号'
    waiting.value = false
    return
  }
  const { data, error: err } = await useApi().GET('/orders/{orderNo}', { params: { path: { orderNo: no.value } } })
  if (!data) {
    error.value = errorMessage(err)
    waiting.value = false
    return
  }
  order.value = data
  // 最多等 1 分钟，之后交给用户手动刷新
  if (data.status === 'pending' && ++tries < 30) {
    timer = setTimeout(check, 2000)
    return
  }
  waiting.value = false
  if (data.status === 'paid') {
    useAuth().refresh()
  }
}
onMounted(check)
onBeforeUnmount(() => clearTimeout(timer))

function retry() {
  tries = 0
  waiting.value = true
  check()
}

const detail = computed(() => {
  const o = order.value
  if (!o) return ''
  if (o.grant?.failed) return '付款已收到，但权益没有自动开通，客服会尽快人工处理'
  if (o.type === 'recharge') return `已到账，当前余额 ¥${yuan(o.grant?.balanceAfter ?? 0)}`
  if (o.type === 'membership') return o.grant?.permanent ? '会员已开通，永久有效' : `会员已开通，${dateOnly(o.grant!.expiresAt!)} 到期`
  return '已解锁，可以查看隐藏内容和下载地址了'
})
const next = computed(() => {
  const o = order.value
  if (o?.type === 'post') return { to: postUrl(o.targetId), text: '返回内容页' }
  if (o?.type === 'membership') return { to: '/vip', text: '查看会员权益' }
  return { to: '/user/wallet', text: '查看余额' }
})
</script>

<template>
  <section class="card mx-auto mt-6 max-w-[520px] px-8 py-10 text-center max-md:px-5">
    <template v-if="order?.status === 'paid'">
      <span class="mx-auto grid size-16 place-items-center rounded-full bg-[#E6F8F1] text-success"><Icon name="check" class="size-8" /></span>
      <h1 class="mt-4 text-xl font-bold text-title">支付成功</h1>
      <p class="mt-1.5 text-sm text-muted">{{ detail }}</p>
    </template>
    <template v-else-if="waiting">
      <span class="mx-auto grid size-16 place-items-center rounded-full bg-theme-soft text-theme"><Icon name="clock" class="size-8" /></span>
      <h1 class="mt-4 text-xl font-bold text-title">正在确认支付结果</h1>
      <p class="mt-1.5 text-sm text-muted">通常几秒钟就好，请不要关闭页面</p>
    </template>
    <template v-else-if="order?.status === 'pending'">
      <span class="mx-auto grid size-16 place-items-center rounded-full bg-[#FFF3E3] text-[#D97706]"><Icon name="clock" class="size-8" /></span>
      <h1 class="mt-4 text-xl font-bold text-title">还没有收到付款</h1>
      <p class="mt-1.5 text-sm text-muted">如果已经付款，可能是渠道通知延迟，稍后再查一次；没有付款的话，可以在“我的订单”里继续支付</p>
      <button type="button" class="btn mt-4 bg-jb-blue" @click="retry">再查一次</button>
    </template>
    <template v-else>
      <span class="mx-auto grid size-16 place-items-center rounded-full bg-soft text-muted"><Icon name="x" class="size-8" /></span>
      <h1 class="mt-4 text-xl font-bold text-title">{{ error ? '查不到这个订单' : order?.status === 'refunded' ? '订单已退款' : '订单已关闭' }}</h1>
      <p class="mt-1.5 text-sm text-muted">{{ error || '订单超时未支付，已经自动关闭，需要的话请重新下单' }}</p>
    </template>

    <dl v-if="order" class="mt-6 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 rounded-lg bg-soft px-4 py-3 text-left text-[13.5px]">
      <dt class="text-muted">商品</dt><dd class="truncate text-title">{{ order.title }}</dd>
      <dt class="text-muted">金额</dt><dd class="text-title">¥{{ yuan(order.payAmount) }}</dd>
      <dt class="text-muted">订单号</dt><dd class="text-title tabular-nums">{{ order.orderNo }}</dd>
    </dl>
    <div class="mt-6 flex justify-center gap-2.5">
      <NuxtLink v-if="order?.status === 'paid'" :to="next.to" class="btn bg-jb-blue">{{ next.text }}</NuxtLink>
      <NuxtLink to="/user/orders" class="btn btn-soft">我的订单</NuxtLink>
    </div>
  </section>
</template>
