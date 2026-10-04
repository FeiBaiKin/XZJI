<script setup lang="ts">
import type { Order } from '~/utils/order'

// 订单表格；待支付订单可以继续支付
defineProps<{ orders: Order[] }>()
const emit = defineEmits<{ pay: [orderNo: string] }>()
const nowMs = Date.now()
const payable = (o: Order) => o.status === 'pending' && new Date(o.expiresAt).getTime() > nowMs
</script>

<template>
  <div class="overflow-x-auto">
    <table class="w-full border-collapse text-[13.5px] [&_td]:border-b [&_td]:border-line [&_td]:p-3 [&_td]:whitespace-nowrap [&_th]:bg-soft [&_th]:px-3 [&_th]:py-[9px] [&_th]:text-left [&_th]:font-medium [&_th]:whitespace-nowrap [&_th]:text-muted [&_th:first-child]:rounded-l-md [&_th:last-child]:rounded-r-md [&_tr:last-child_td]:border-b-0">
      <thead>
        <tr><th>订单号</th><th>商品</th><th>金额</th><th>支付方式</th><th>状态</th><th>时间</th></tr>
      </thead>
      <tbody class="text-title">
        <tr v-for="o in orders" :key="o.orderNo">
          <td class="text-muted tabular-nums">{{ o.orderNo }}</td>
          <td class="max-w-[16em] truncate" :title="o.title">
            <NuxtLink v-if="o.type === 'post'" :to="postUrl(o.targetId)" class="hover:text-theme">{{ o.title }}</NuxtLink>
            <template v-else>{{ o.title }}</template>
          </td>
          <td class="font-semibold tabular-nums">¥{{ yuan(o.payAmount) }}</td>
          <td>{{ payMethodName[o.payMethod] ?? '—' }}</td>
          <td>
            <span class="inline-block rounded px-2 py-1 text-xs leading-none" :class="orderStatus[o.status]!.class">{{ o.status === 'closed' ? '已关闭' : orderStatus[o.status]!.text }}</span>
            <button v-if="payable(o)" type="button" class="ml-2 text-theme hover:underline" @click="emit('pay', o.orderNo)">去支付</button>
          </td>
          <td class="text-muted tabular-nums">{{ shortTime(o.createdAt) }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
