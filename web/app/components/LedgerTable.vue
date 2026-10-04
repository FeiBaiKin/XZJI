<script setup lang="ts">
import type { PublicSchemas } from '@xzji/api-client/public'

// 余额明细表格
defineProps<{ entries: PublicSchemas['LedgerEntry'][] }>()
</script>

<template>
  <div class="overflow-x-auto">
    <table class="w-full border-collapse text-[13.5px] [&_td]:border-b [&_td]:border-line [&_td]:p-3 [&_td]:whitespace-nowrap [&_th]:bg-soft [&_th]:px-3 [&_th]:py-[9px] [&_th]:text-left [&_th]:font-medium [&_th]:whitespace-nowrap [&_th]:text-muted [&_th:first-child]:rounded-l-md [&_th:last-child]:rounded-r-md [&_tr:last-child_td]:border-b-0">
      <thead>
        <tr><th>时间</th><th>类型</th><th>说明</th><th>变动</th><th>变动后余额</th></tr>
      </thead>
      <tbody class="text-title">
        <tr v-for="e in entries" :key="e.id">
          <td class="text-muted tabular-nums">{{ shortTime(e.createdAt) }}</td>
          <td>{{ ledgerType[e.bizType] }}</td>
          <td class="max-w-[20em] truncate" :title="e.remark">{{ e.remark }}</td>
          <td class="font-semibold tabular-nums" :class="e.amount > 0 ? 'text-[#0E9F78]' : 'text-danger'">{{ signedYuan(e.amount) }}</td>
          <td class="text-muted tabular-nums">{{ yuan(e.balanceAfter) }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
