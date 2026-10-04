<script setup lang="ts">
import type { PublicSchemas } from '@xzji/api-client/public'

definePageMeta({ middleware: 'auth' })

const route = useRoute()
const page = computed(() => pageParam(route.query.page))
const { overview, refresh } = useAccount()
const data = ref<PublicSchemas['LedgerPage'] | null>(null)

async function load() {
  const { data: res } = await useApi().GET('/me/wallet/ledger', { params: { query: { page: page.value, pageSize: 20 } } })
  data.value = res ?? null
}
watch(page, load, { immediate: true })

// 充值金额（元）：常用档位或自己填
const presets = [10, 30, 50, 100, 200]
const amount = ref(30)
const custom = ref('')
const yuanValue = computed(() => custom.value ? Number(custom.value) : amount.value)
const valid = computed(() => (!custom.value || /^\d+(\.\d{1,2})?$/.test(custom.value)) && yuanValue.value >= 1 && yuanValue.value <= 5000)

const paying = ref(false)
async function onPaid() {
  await Promise.all([refresh(), load()])
}
</script>

<template>
  <UserShell title="我的钱包">
    <section class="card px-5 py-4 max-md:px-3.5">
      <h2 class="sec-title">余额充值</h2>
      <div class="mt-4 flex flex-wrap items-end gap-x-8 gap-y-4">
        <div>
          <span class="text-[13px] text-muted">当前余额</span>
          <b class="block text-[28px] leading-tight font-extrabold text-title">¥{{ yuan(overview?.balance ?? 0) }}</b>
        </div>
        <p class="text-[12.5px] text-muted">余额可以购买付费资源、开通会员，不能提现</p>
      </div>
      <div class="mt-4 flex flex-wrap gap-2.5" role="radiogroup" aria-label="充值金额">
        <button
          v-for="v in presets"
          :key="v"
          type="button"
          role="radio"
          :aria-checked="!custom && amount === v"
          class="h-11 min-w-[88px] rounded-lg border-[1.5px] px-4 text-[15px] font-semibold"
          :class="!custom && amount === v ? 'border-theme bg-theme-soft text-theme' : 'border-line bg-white text-title hover:border-[#D5DAE1]'"
          @click="amount = v; custom = ''"
        >
          ¥{{ v }}
        </button>
        <label class="relative">
          <span class="sr-only">其他金额</span>
          <input v-model.trim="custom" inputmode="decimal" placeholder="其他金额" class="field-input h-11 w-[132px] pr-8">
          <span class="absolute top-1/2 right-3 -translate-y-1/2 text-muted">元</span>
        </label>
      </div>
      <p v-if="custom && !valid" class="mt-2 text-[13px] text-danger">充值金额需要在 1 到 5000 元之间，最多两位小数</p>
      <button type="button" class="btn mt-4 h-11 bg-jb-blue px-8 text-[15px]" :disabled="!valid" @click="paying = true">充值 ¥{{ valid ? yuanValue.toFixed(2) : '—' }}</button>
    </section>

    <section class="card mt-5 px-5 py-4 max-md:px-3.5">
      <h2 class="sec-title mb-3">余额明细</h2>
      <template v-if="data">
        <LedgerTable v-if="data.items.length" :entries="data.items" />
        <p v-else class="py-12 text-center text-[13px] text-muted">还没有余额变动</p>
      </template>
      <p v-else class="py-12 text-center text-[13px] text-muted">正在加载…</p>
    </section>
    <PagerNav v-if="data" :page="data.page" :total="data.total" :page-size="data.pageSize" :to="p => ({ query: { page: p } })" />

    <PayDialog v-model="paying" type="recharge" :amount="Math.round(yuanValue * 100)" @paid="onPaid" />
  </UserShell>
</template>
