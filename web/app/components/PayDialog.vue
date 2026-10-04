<script setup lang="ts">
import { renderSVG } from 'uqr'
import { errorCode, errorMessage } from '@xzji/api-client/public'
import type { Order, OrderQuote, OrderType, PayAction, PayMethod } from '~/utils/order'

// 支付弹窗：新下单传 type + targetId（充值传 amount），继续支付已有订单传 orderNo
const props = defineProps<{
  type?: OrderType
  targetId?: number
  amount?: number
  orderNo?: string
  // 底部提示，如“开通会员可以免费下载”
  hint?: { text: string, link: string, to: string }
}>()
const open = defineModel<boolean>({ required: true })
const emit = defineEmits<{ paid: [order: Order] }>()

const api = useApi()
const toast = useToast()

const quote = ref<OrderQuote | null>(null)
const order = ref<Order | null>(null)
const action = ref<PayAction | null>(null)
const method = ref<PayMethod>('wechat')
const error = ref('')
const loading = ref(false)
const busy = ref(false)
const done = ref(false)
const now = ref(Date.now())
let clock: ReturnType<typeof setInterval> | undefined
let poller: ReturnType<typeof setTimeout> | undefined

const kind = computed<OrderType>(() => order.value?.type ?? props.type ?? 'post')
// 金额以订单为准；还没下单时用报价
const view = computed(() => {
  const src = order.value ?? quote.value
  if (!src) return null
  return { title: src.title, amount: src.amount, discount: src.discountAmount, pay: src.payAmount, membership: src.membership }
})
const balance = computed(() => quote.value?.balance ?? 0)
const methods = computed(() => (quote.value?.methods ?? []).map((m) => {
  // 继续支付时按订单金额重新判断余额是否足够
  if (m.method === 'balance' && m.available && view.value && balance.value < view.value.pay) {
    return { ...m, available: false, reason: '余额不足' }
  }
  return m
}))
const current = computed(() => methods.value.find(m => m.method === method.value))

const left = computed(() => {
  const s = order.value ? Math.max(0, Math.floor((new Date(order.value.expiresAt).getTime() - now.value) / 1000)) : 1800
  return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`
})

const tile = computed(() => {
  if (kind.value === 'recharge') return { icon: 'wallet', class: 'bg-jb-orange' }
  if (kind.value === 'membership') {
    const rank = view.value?.membership?.rank ?? 1
    return rank > 1 ? { icon: 'gem', class: 'bg-jb-purple' } : { icon: 'crown', class: 'bg-[linear-gradient(135deg,#F7D58B,#E0A63E)] text-[#4A2F00]' }
  }
  return { icon: 'box', class: 'bg-jb-blue' }
})
const note = computed(() => {
  if (kind.value === 'recharge') return '余额只能用于站内消费，不能提现'
  if (view.value?.membership) return membershipNote(view.value.membership)
  return '购买后永久有效，可以在“已购资源”里随时重新下载'
})
const discountLabel = computed(() => kind.value === 'membership' ? '升级抵扣' : '会员折扣')
const qrSvg = computed(() => action.value?.qrCode ? renderSVG(action.value.qrCode, { border: 0 }) : '')
const channelName = computed(() => method.value === 'alipay' ? '支付宝' : '微信')

function stopPoll() {
  clearTimeout(poller)
  poller = undefined
}

function finish(o: Order) {
  stopPoll()
  order.value = o
  done.value = true
  action.value = null
  emit('paid', o)
}

// 轮询订单状态：后端每次会顺便向渠道查询一次
function startPoll() {
  stopPoll()
  const tick = async () => {
    if (!open.value || !order.value) return
    const { data } = await api.GET('/orders/{orderNo}', { params: { path: { orderNo: order.value.orderNo } } })
    if (data?.status === 'paid') return finish(data)
    if (data && data.status !== 'pending') {
      order.value = null
      action.value = null
      error.value = '订单已关闭，请重新下单'
      return
    }
    poller = setTimeout(tick, 2000)
  }
  poller = setTimeout(tick, 2000)
}

async function load() {
  quote.value = order.value = action.value = null
  error.value = ''
  done.value = false
  loading.value = true
  try {
    let type = props.type
    let targetId = props.targetId
    let amount = props.amount
    if (props.orderNo) {
      const { data, error: err } = await api.GET('/orders/{orderNo}', { params: { path: { orderNo: props.orderNo } } })
      if (!data) {
        error.value = errorMessage(err)
        return
      }
      if (data.status !== 'pending') {
        error.value = data.status === 'paid' ? '这个订单已经支付过了' : '订单已关闭，请重新下单'
        return
      }
      order.value = data
      type = data.type
      targetId = data.targetId
      amount = data.type === 'recharge' ? data.payAmount : undefined
    }
    const { data, error: err } = await api.GET('/orders/quote', {
      params: { query: { type: type!, targetId: targetId || undefined, amount, scene: payScene() } },
    })
    if (!data) {
      error.value = errorMessage(err)
      return
    }
    quote.value = data
    const pick = methods.value.find(m => m.method === 'balance' && m.available)
      ?? methods.value.find(m => m.method === order.value?.payMethod && m.available)
      ?? methods.value.find(m => m.available)
    if (pick) method.value = pick.method
  } finally {
    loading.value = false
  }
}

function choose(m: PayMethod) {
  if (m === method.value) return
  method.value = m
  action.value = null
  stopPoll()
}

async function pay() {
  if (!view.value || !current.value?.available) return
  busy.value = true
  error.value = ''
  const scene = payScene()
  try {
    const res = order.value
      ? await api.POST('/orders/{orderNo}/pay', { params: { path: { orderNo: order.value.orderNo } }, body: { payMethod: method.value, scene } })
      : await api.POST('/orders', { body: { type: props.type!, targetId: props.targetId || undefined, amount: props.amount, payMethod: method.value, scene } })
    if (!res.data) {
      if (errorCode(res.error) === 'ORDER_NOT_PENDING') order.value = null
      error.value = errorMessage(res.error)
      return
    }
    order.value = res.data.order
    const act = res.data.action
    if (act.kind === 'paid') return finish(res.data.order)
    action.value = act
    if (act.kind === 'redirect' && scene === 'mobile') {
      location.href = act.url!
      return
    }
    startPoll()
  } finally {
    busy.value = false
  }
}

async function mockPay() {
  if (!order.value) return
  busy.value = true
  const { data, error: err } = await api.POST('/orders/{orderNo}/mock-pay', { params: { path: { orderNo: order.value.orderNo } } })
  busy.value = false
  if (data?.status === 'paid') return finish(data)
  toast.error(errorMessage(err))
}

const doneText = computed(() => {
  const o = order.value
  if (!o) return ''
  if (o.grant?.failed) return '付款已收到，但权益没有自动开通，客服会尽快人工处理'
  if (o.type === 'recharge') return `已到账，当前余额 ¥${yuan(o.grant?.balanceAfter ?? 0)}`
  if (o.type === 'membership') return o.grant?.permanent ? '会员已开通，永久有效' : `会员已开通，${dateOnly(o.grant!.expiresAt!)} 到期`
  return '已解锁，可以查看隐藏内容和下载地址了'
})

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') open.value = false
}

function onOpen(v: boolean) {
  if (v) {
    load()
    clock = setInterval(() => { now.value = Date.now() }, 1000)
    addEventListener('keydown', onKey)
  } else {
    stopPoll()
    clearInterval(clock)
    removeEventListener('keydown', onKey)
  }
}
// 只在浏览器端打开，服务端渲染时不挂任何监听
watch(open, onOpen)
onMounted(() => {
  if (open.value) onOpen(true)
})
onBeforeUnmount(() => {
  stopPoll()
  clearInterval(clock)
  removeEventListener('keydown', onKey)
})
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-60 grid place-items-center bg-[rgba(20,22,28,.5)] p-4" @click.self="open = false">
      <div class="card relative max-h-[calc(100vh-32px)] w-full max-w-[460px] overflow-auto p-[22px]" role="dialog" aria-modal="true" aria-labelledby="pay-title">
        <button type="button" class="absolute top-3 right-3 grid size-8 place-items-center rounded-full text-muted hover:bg-soft hover:text-title" aria-label="关闭" @click="open = false">
          <Icon name="x" class="size-[18px]" />
        </button>

        <template v-if="done && order">
          <div class="flex flex-col items-center py-4 text-center">
            <span class="grid size-14 place-items-center rounded-full bg-[#E6F8F1] text-success"><Icon name="check" class="size-7" /></span>
            <b id="pay-title" class="mt-3 text-lg text-title">支付成功</b>
            <p class="mt-1 text-[13.5px] text-muted">{{ doneText }}</p>
            <p class="mt-3 text-[12.5px] text-muted">订单号 {{ order.orderNo }}</p>
            <button type="button" class="btn mt-5 h-10 w-full bg-jb-blue" @click="open = false">完成</button>
          </div>
        </template>

        <template v-else>
          <div class="flex flex-wrap items-baseline gap-x-2.5 gap-y-1 pr-[30px]">
            <b id="pay-title" class="text-[17px] text-title">确认订单</b>
            <span class="text-[12.5px] text-muted">订单将在 <em class="font-semibold text-danger not-italic">{{ left }}</em> 后自动关闭</span>
          </div>

          <div v-if="loading" class="py-14 text-center text-[13px] text-muted">正在计算价格…</div>
          <p v-else-if="!view" class="mt-4 rounded-md bg-danger-soft px-3 py-2.5 text-[13.5px] text-danger">{{ error || '暂时无法下单，请稍后再试' }}</p>

          <template v-else>
            <div class="mt-4 flex items-center gap-3 rounded-lg bg-soft p-3">
              <span class="grid size-11 shrink-0 place-items-center rounded-[11px] text-white" :class="tile.class"><Icon :name="tile.icon" class="size-[22px]" /></span>
              <div class="min-w-0">
                <b class="block truncate text-[14.5px] leading-[1.4] text-title">{{ view.title }}</b>
                <small class="mt-0.5 block text-xs leading-[1.55] text-muted">{{ note }}</small>
              </div>
            </div>

            <dl class="mt-3.5 grid grid-cols-[1fr_auto] gap-1.5 text-[13.5px]">
              <dt class="text-muted">{{ kind === 'recharge' ? '充值金额' : kind === 'membership' ? '套餐价格' : '原价' }}</dt>
              <dd class="text-right text-title">¥{{ yuan(view.amount) }}</dd>
              <template v-if="view.discount > 0">
                <dt class="text-muted">{{ discountLabel }}</dt>
                <dd class="text-right text-title">-¥{{ yuan(view.discount) }}</dd>
              </template>
              <dt class="mt-0.5 self-center border-t border-dashed border-line pt-2 text-title">应付金额</dt>
              <dd class="mt-0.5 border-t border-dashed border-line pt-2 text-right text-[22px] leading-[1.2] font-extrabold text-danger">¥{{ yuan(view.pay) }}</dd>
            </dl>

            <div class="mt-4 grid grid-cols-3 gap-2.5" role="radiogroup" aria-label="支付方式">
              <button
                v-for="m in methods"
                :key="m.method"
                type="button"
                role="radio"
                :aria-checked="method === m.method"
                :disabled="!m.available || busy"
                class="flex flex-col items-center gap-1 rounded-lg border-[1.5px] px-1 py-2.5 text-[13px] text-title disabled:cursor-not-allowed disabled:opacity-55"
                :class="method === m.method && m.available ? 'border-theme bg-theme-soft' : 'border-line bg-white'"
                @click="choose(m.method)"
              >
                <i class="grid size-[26px] place-items-center rounded-[7px] text-[13px] font-bold text-white not-italic" :class="{ 'bg-[#16B35A]': m.method === 'wechat', 'bg-[#1677FF]': m.method === 'alipay', 'bg-jb-orange': m.method === 'balance' }">{{ { wechat: '微', alipay: '支', balance: '余' }[m.method] }}</i>
                {{ m.method === 'wechat' ? '微信支付' : m.method === 'alipay' ? '支付宝' : '余额支付' }}
                <small v-if="m.method === 'balance' && (!m.reason || m.reason === '余额不足')" class="text-[11px] leading-[1.3] text-muted">余额 ¥{{ yuan(balance) }}{{ m.reason ? '，不足' : '' }}</small>
                <small v-else-if="m.reason" class="text-[11px] leading-[1.3] text-muted">{{ m.reason }}</small>
              </button>
            </div>

            <div v-if="method === 'balance' && current?.available" class="mt-4 rounded-lg bg-soft px-3.5 py-3 text-[13.5px]">
              <div class="flex justify-between"><span>当前余额</span><b class="text-title">¥{{ yuan(balance) }}</b></div>
              <div class="mt-1 flex justify-between"><span>支付后余额</span><b class="text-title">¥{{ yuan(balance - view.pay) }}</b></div>
            </div>

            <div v-if="action?.kind === 'qrcode'" class="mt-4 flex items-center gap-4 rounded-lg border border-dashed border-[#DDE1E7] p-3.5">
              <!-- eslint-disable-next-line vue/no-v-html -- 二维码 SVG 由本地库生成 -->
              <div class="size-[124px] shrink-0 rounded-md border border-line bg-white p-2 [&_svg]:size-full" role="img" :aria-label="`${channelName}付款二维码`" v-html="qrSvg" />
              <div>
                <b class="block text-[14.5px] text-title">打开{{ channelName }}扫一扫</b>
                <p class="mt-0.5 text-[13px] text-muted">支付金额 <em class="text-lg font-extrabold text-danger not-italic">¥{{ yuan(view.pay) }}</em></p>
                <p class="mt-0.5 text-[13px] text-muted">付款后页面会自动刷新，不用手动操作</p>
                <button v-if="action.mock" type="button" class="btn mt-2 h-8 bg-jb-green px-3 text-[13px]" :disabled="busy" @click="mockPay">模拟支付成功</button>
              </div>
            </div>
            <div v-else-if="action?.kind === 'redirect'" class="mt-4 rounded-lg border border-dashed border-[#DDE1E7] p-3.5 text-center">
              <a :href="action.url" target="_blank" rel="noopener noreferrer" class="btn h-10 w-full bg-[#1677FF]">打开{{ channelName }}付款页面</a>
              <p class="mt-2 text-[13px] text-muted">在新页面付款完成后，这里会自动刷新</p>
            </div>

            <p v-if="error" class="mt-3 rounded-md bg-danger-soft px-3 py-2 text-[13px] text-danger">{{ error }}</p>

            <button v-if="!action" type="button" class="btn mt-3.5 h-11 w-full bg-jb-red text-[15px]" :disabled="busy || !current?.available" @click="pay">
              {{ busy ? '正在提交…' : `确认支付 ¥${yuan(view.pay)}` }}
            </button>

            <p class="mt-3.5 text-center text-xs text-muted">
              <template v-if="hint">{{ hint.text }}，<NuxtLink :to="hint.to" class="text-theme" @click="open = false">{{ hint.link }}</NuxtLink></template>
              <template v-else-if="kind === 'membership'">虚拟商品开通后不支持线上退款，遇到重复扣款请联系客服处理</template>
              <template v-else-if="kind === 'recharge'">充值后可以用余额购买资源和开通会员</template>
            </p>
          </template>
        </template>
      </div>
    </div>
  </Teleport>
</template>
