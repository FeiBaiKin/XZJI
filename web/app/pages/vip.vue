<script setup lang="ts">
import { errorMessage } from '@xzji/api-client/public'
import type { LevelPlans, MembershipPlan, OrderQuote } from '~/utils/order'

const api = useApi()
const route = useRoute()
const siteName = useRuntimeConfig().public.siteName

// 等级和套餐是公开内容，服务端渲染；当前用户的续费、升级报价在浏览器端获取
const { data } = await useAsyncData('membership:plans', async () => {
  const { data } = await api.GET('/membership/plans')
  return data?.levels ?? []
}, { default: () => [] as LevelPlans[] })
const levels = computed(() => data.value.filter(l => l.plans.length))

useSeoMeta({
  title: useSiteTitle('开通会员'),
  description: () => `${siteName} 会员：${levels.value.map(l => l.name).join('、')}，会员资源免费下载，付费资源享受折扣。`,
})

const auth = useAuth()
const user = computed(() => auth.user.value)
const current = computed(() => user.value?.membership)

const discountText = (rate: number) => String(rate / 10)
const minDiscount = computed(() => Math.min(100, ...levels.value.map(l => l.discountRate)))

// 默认选中带角标的套餐，其次年卡，再其次第一个
const picked = ref<Record<number, number>>({})
watchEffect(() => {
  for (const l of levels.value) {
    if (l.plans.some(p => p.id === picked.value[l.id])) continue
    const p = l.plans.find(p => p.badge) ?? l.plans.find(p => p.durationDays === 365) ?? l.plans[0]!
    picked.value[l.id] = p.id
  }
})
const planOf = (l: LevelPlans) => l.plans.find(p => p.id === picked.value[l.id]) ?? l.plans[0]!

function perUnit(p: MembershipPlan) {
  if (p.durationDays === 0) return '一次付费'
  if (p.durationDays === 365) return `每月 ¥${(p.price / 1200).toFixed(1)}`
  return `每天 ¥${(p.price / p.durationDays / 100).toFixed(2)}`
}

// 高等级自动包含低等级的自定义权益；更高等级才有的显示为灰色
function perksOf(index: number) {
  const l = levels.value[index]!
  const own = new Set(levels.value.slice(0, index + 1).flatMap(x => x.perks))
  const higher = [...new Set(levels.value.slice(index + 1).flatMap(x => x.perks))].filter(p => !own.has(p))
  return [
    { text: `${l.name} 资源免费下载`, yes: true },
    l.discountRate < 100 ? { text: `付费资源 ${discountText(l.discountRate)} 折`, yes: true } : { text: '付费资源原价购买', yes: false },
    { text: l.dailyDownloadLimit > 0 ? `每天 ${l.dailyDownloadLimit} 次免费下载` : '免费下载不限次数', yes: true },
    ...[...own].map(text => ({ text, yes: true })),
    ...higher.map(text => ({ text, yes: false })),
  ]
}

type QuoteState = { loading: boolean, data?: OrderQuote, error?: string }
const quotes = ref<Record<number, QuoteState>>({})

async function loadQuote(l: LevelPlans) {
  const plan = planOf(l)
  quotes.value[l.id] = { loading: true }
  const { data, error } = await api.GET('/orders/quote', { params: { query: { type: 'membership', targetId: plan.id, scene: payScene() } } })
  if (planOf(l).id !== plan.id) return
  quotes.value[l.id] = data ? { loading: false, data } : { loading: false, error: errorMessage(error) }
}
function loadQuotes() {
  if (user.value) levels.value.forEach(loadQuote)
}
onMounted(async () => {
  await auth.ensure()
  loadQuotes()
})

function pick(l: LevelPlans, p: MembershipPlan) {
  if (picked.value[l.id] === p.id) return
  picked.value[l.id] = p.id
  if (user.value) loadQuote(l)
}

function buttonText(l: LevelPlans) {
  const plan = planOf(l)
  const q = quotes.value[l.id]
  const m = q?.data?.membership
  const verb = m?.kind === 'renew' ? '续费' : m?.kind === 'upgrade' ? '升级' : '开通'
  return `${verb} ${l.name} ${plan.name}，¥${yuan(q?.data?.payAmount ?? plan.price)}`
}

const paying = ref(false)
const payPlan = ref(0)
function buy(l: LevelPlans) {
  if (!user.value) return navigateTo({ path: '/login', query: { redirect: route.fullPath } })
  const q = quotes.value[l.id]
  if (!q?.data) return
  payPlan.value = planOf(l).id
  paying.value = true
}
async function onPaid() {
  await auth.refresh()
  loadQuotes()
}

const nowText = computed(() => {
  const m = current.value
  if (!user.value) return ''
  if (!m) return '你还不是会员，开通后立即生效'
  if (!m.expiresAt) return `你当前是永久 ${m.name}`
  return `你当前是 ${m.name}，${dateOnly(m.expiresAt)} 到期，还剩 ${daysLeft(m.expiresAt)} 天`
})

// 权益对比表
const customAll = computed(() => [...new Set(levels.value.flatMap(l => l.perks))])
const hasPerk = (index: number, perk: string) => levels.value.slice(0, index + 1).some(l => l.perks.includes(perk))

const faqs = computed(() => {
  const [a, b] = levels.value
  return [
    {
      q: b ? `升级到 ${b.name}，原来剩下的 ${a!.name} 时长怎么算？` : '升级到更高等级，原来剩下的时长怎么算？',
      a: '按剩余价值折算：剩余天数 × 原等级日均价（年卡价 ÷ 365），再按新等级的日均价换算成天数，叠加到新开通的套餐上；升级到永久套餐时，剩余价值直接抵扣价格。永久会员升级到更高等级的永久套餐，只需要补两者永久价的差价。',
    },
    {
      q: '每天的免费下载次数怎么算？',
      a: '只统计靠会员身份免费下载的次数，同一个资源当天重复下载只算一次。单独购买过的资源和免费资源不占次数，次数每天 0 点重置。',
    },
    {
      q: '续费会员，到期时间怎么算？',
      a: '同等级续费时，在原到期时间上直接叠加套餐时长，剩余的天数不会浪费。',
    },
    {
      q: '会员到期后，已经下载的资源还能用吗？',
      a: '能用。到期后不能再免费下载会员资源，但单独购买过的资源永久有效，随时可以在“已购资源”里重新下载。',
    },
    {
      q: '开通后可以退款吗？',
      a: '会员属于虚拟商品，开通后不支持线上退款。如果遇到重复扣款等问题，请联系客服，我们会人工核实处理。',
    },
  ]
})
</script>

<template>
  <div class="pb-2">
    <section class="relative overflow-hidden rounded-card bg-jb-vip px-10 pt-11 pb-24 text-vip after:absolute after:-top-20 after:right-[10%] after:size-[340px] after:rounded-full after:bg-[radial-gradient(circle,rgba(246,199,107,.22),transparent_65%)] max-md:px-5 max-md:pt-8">
      <h1 class="relative z-10 flex items-center gap-2.5 text-[30px] leading-[1.3] font-bold text-[#F6D48F] max-md:text-2xl">
        <Icon name="crown" class="size-8" />{{ siteName }} 会员
      </h1>
      <p class="relative z-10 mt-1.5 text-[15px] opacity-80">
        会员资源免费下载<template v-if="minDiscount < 100">，付费资源最低 {{ discountText(minDiscount) }} 折</template>
      </p>
      <ClientOnly>
        <p v-if="nowText" class="relative z-10 mt-4 inline-flex rounded-full bg-white/10 px-3.5 py-1.5 text-[13px]">{{ nowText }}</p>
      </ClientOnly>
    </section>

    <div v-if="levels.length" class="relative z-10 -mt-16 grid gap-5 px-5 max-md:px-0" :class="levels.length >= 3 ? 'grid-cols-3 max-[1100px]:grid-cols-2 max-md:grid-cols-1' : 'grid-cols-2 max-md:grid-cols-1'">
      <section v-for="(l, i) in levels" :key="l.id" class="card flex flex-col overflow-hidden">
        <div class="flex items-center gap-2.5 px-[22px] py-[18px]" :class="levelTone(i).head">
          <Icon :name="levelTone(i).icon" class="size-6" />
          <b class="text-xl">{{ l.name }}</b>
          <span v-if="l.description" class="ml-auto text-[13px] opacity-85">{{ l.description }}</span>
        </div>
        <div class="flex flex-1 flex-col p-[22px] max-md:p-4">
          <div class="grid gap-3 pt-1" :style="{ gridTemplateColumns: `repeat(${Math.min(l.plans.length, 3)}, minmax(0, 1fr))` }" role="radiogroup" :aria-label="`${l.name} 套餐`">
            <button
              v-for="p in l.plans"
              :key="p.id"
              type="button"
              role="radio"
              :aria-checked="planOf(l).id === p.id"
              class="relative flex flex-col items-center rounded-card border-[1.5px] px-1.5 pt-4 pb-3 text-center transition-colors"
              :class="planOf(l).id === p.id ? levelTone(i).on : 'border-line bg-white hover:border-[#D5DAE1]'"
              @click="pick(l, p)"
            >
              <em v-if="p.badge" class="absolute -top-2.5 left-1/2 -translate-x-1/2 rounded-full bg-jb-red px-2 py-1 text-[11.5px] leading-none font-semibold whitespace-nowrap text-white not-italic">{{ p.badge }}</em>
              <span class="text-[13.5px]">{{ p.name }}</span>
              <b class="text-[26px] leading-[1.3]" :class="planOf(l).id === p.id ? levelTone(i).price : 'text-title'"><small class="text-sm">¥</small>{{ p.price / 100 }}</b>
              <del v-if="p.originalPrice" class="text-xs text-muted">¥{{ p.originalPrice / 100 }}</del>
              <span class="mt-0.5 text-[11.5px] text-muted">{{ perUnit(p) }}</span>
            </button>
          </div>

          <ul class="mt-5 grid grid-cols-2 gap-x-4 gap-y-[9px] text-[13.5px] max-md:grid-cols-1">
            <li v-for="perk in perksOf(i)" :key="perk.text" class="flex items-center gap-1.5" :class="perk.yes ? '' : 'text-muted'">
              <Icon :name="perk.yes ? 'check' : 'minus'" class="size-[15px]" :class="perk.yes ? 'text-success' : 'text-[#C5CBD3]'" />{{ perk.text }}
            </li>
          </ul>

          <ClientOnly>
            <template v-if="quotes[l.id]?.data?.membership && quotes[l.id]!.data!.membership!.kind !== 'new'">
              <p class="mt-[18px] rounded-lg px-3.5 py-3 text-[13px] leading-[1.75]" :class="levelTone(i).note">{{ membershipNote(quotes[l.id]!.data!.membership!) }}</p>
            </template>
            <p v-else-if="quotes[l.id]?.error" class="mt-[18px] rounded-lg bg-soft px-3.5 py-3 text-[13px] leading-[1.75] text-muted">{{ quotes[l.id]!.error }}</p>
          </ClientOnly>

          <span class="min-h-[18px] flex-1" />
          <button
            type="button"
            class="btn h-11 w-full text-[15px]"
            :class="levelTone(i).btn"
            :disabled="!!user && (quotes[l.id]?.loading || !!quotes[l.id]?.error)"
            @click="buy(l)"
          >
            {{ buttonText(l) }}
          </button>
        </div>
      </section>
    </div>
    <p v-else class="card mt-5 px-5 py-14 text-center text-muted">会员套餐还在准备中，敬请期待</p>

    <section v-if="levels.length" class="card mt-5 px-5 pt-4 pb-3 max-md:px-3">
      <h2 class="sec-title mb-3">权益对比</h2>
      <div class="overflow-x-auto">
        <table class="w-full border-collapse text-center text-[13.5px] [&_td]:border-b [&_td]:border-line [&_td]:px-3 [&_td]:py-3 [&_td]:whitespace-nowrap [&_td:first-child]:text-left [&_th]:bg-soft [&_th]:px-3 [&_th]:py-[9px] [&_th]:font-medium [&_th]:whitespace-nowrap [&_th]:text-muted [&_th:first-child]:rounded-l-md [&_th:first-child]:text-left [&_th:last-child]:rounded-r-md [&_tr:last-child_td]:border-b-0">
          <thead>
            <tr>
              <th>权益</th>
              <th>普通用户</th>
              <th v-for="l in levels" :key="l.id">{{ l.name }}</th>
            </tr>
          </thead>
          <tbody class="text-title">
            <tr>
              <td>免费资源</td>
              <td class="font-semibold text-success">可以下载</td>
              <td v-for="l in levels" :key="l.id" class="font-semibold text-success">可以下载</td>
            </tr>
            <tr v-for="(row, ri) in levels" :key="row.id">
              <td>{{ row.name }} 资源</td>
              <td class="text-[#B7BDC6]">{{ ri === 0 ? '单独购买' : '无' }}</td>
              <td v-for="l in levels" :key="l.id" :class="l.rank >= row.rank ? 'font-semibold text-success' : 'text-[#B7BDC6]'">{{ l.rank >= row.rank ? '免费下载' : '无' }}</td>
            </tr>
            <tr>
              <td>付费资源</td>
              <td class="text-[#B7BDC6]">原价</td>
              <td v-for="l in levels" :key="l.id">{{ l.discountRate < 100 ? `${discountText(l.discountRate)} 折` : '原价' }}</td>
            </tr>
            <tr>
              <td>每天免费下载次数</td>
              <td class="text-[#B7BDC6]">无</td>
              <td v-for="l in levels" :key="l.id">{{ l.dailyDownloadLimit > 0 ? `${l.dailyDownloadLimit} 次` : '不限' }}</td>
            </tr>
            <tr v-for="perk in customAll" :key="perk">
              <td>{{ perk }}</td>
              <td class="text-[#B7BDC6]">无</td>
              <td v-for="(l, i) in levels" :key="l.id" :class="hasPerk(i, perk) ? 'font-semibold text-success' : 'text-[#B7BDC6]'">{{ hasPerk(i, perk) ? '有' : '无' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="card mt-5 px-5 py-1 max-md:px-3.5">
      <details v-for="(f, i) in faqs" :key="f.q" :open="i === 0" class="group border-b border-line last:border-b-0">
        <summary class="flex cursor-pointer list-none items-center py-3.5 text-[14.5px] font-semibold text-title [&::-webkit-details-marker]:hidden">
          {{ f.q }}
          <Icon name="right" class="ml-auto size-4 text-muted transition-transform group-open:rotate-90" />
        </summary>
        <p class="pb-3.5 text-[13.5px] leading-[1.85]">{{ f.a }}</p>
      </details>
    </section>

    <PayDialog v-model="paying" type="membership" :target-id="payPlan" @paid="onPaid" />
  </div>
</template>
