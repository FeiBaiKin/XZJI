<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { errorMessage, type AdminSchemas } from '@xzji/api-client/admin'
import { api, formatTime } from '../api'
import { yuan } from '../content'
import { bizTypeName, signedYuan } from '../trade'

type Entry = AdminSchemas['AdminLedgerEntry']
type BizType = AdminSchemas['LedgerBizType']

const route = useRoute()
const filter = reactive<{ userId?: number, bizType: BizType | '' }>({ userId: Number(route.query.userId) || undefined, bizType: '' })
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const rows = ref<Entry[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  const { data, error } = await api.GET('/ledger', {
    params: { query: { userId: filter.userId || undefined, bizType: filter.bizType || undefined, page: page.value, pageSize: pageSize.value } },
  })
  loading.value = false
  if (!data) return ElMessage.error(errorMessage(error))
  rows.value = data.items
  total.value = data.total
}

function search() {
  page.value = 1
  load()
}

onMounted(load)
</script>

<template>
  <el-card shadow="never">
    <el-form inline class="filter" @submit.prevent="search">
      <el-form-item>
        <el-input-number v-model="filter.userId" placeholder="用户 ID" :min="1" :controls="false" style="width: 140px" />
      </el-form-item>
      <el-form-item>
        <el-select v-model="filter.bizType" placeholder="全部类型" clearable style="width: 130px" @change="search">
          <el-option v-for="(label, v) in bizTypeName" :key="v" :label="label" :value="v" />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" native-type="submit">搜索</el-button>
      </el-form-item>
    </el-form>
    <p class="tip">流水只增不改，是余额的唯一依据。调整余额请到“用户管理”，退款请到“订单管理”。</p>

    <el-table v-loading="loading" :data="rows" border stripe empty-text="没有流水">
      <el-table-column label="时间" width="150">
        <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="用户" min-width="180">
        <template #default="{ row }">
          <div class="who"><b>{{ row.user.nickname }}</b><small>ID {{ row.user.id }}，{{ row.user.email }}</small></div>
        </template>
      </el-table-column>
      <el-table-column label="类型" width="100">
        <template #default="{ row }">{{ bizTypeName[row.bizType as BizType] }}</template>
      </el-table-column>
      <el-table-column prop="remark" label="说明" min-width="220" show-overflow-tooltip />
      <el-table-column label="变动" width="110">
        <template #default="{ row }"><b :class="row.amount > 0 ? 'plus' : 'minus'">{{ signedYuan(row.amount) }}</b></template>
      </el-table-column>
      <el-table-column label="变动后余额" width="120">
        <template #default="{ row }">{{ yuan(row.balanceAfter) }}</template>
      </el-table-column>
    </el-table>

    <el-pagination
      v-model:current-page="page"
      v-model:page-size="pageSize"
      class="pager"
      :total="total"
      :page-sizes="[20, 50, 100]"
      layout="total, sizes, prev, pager, next"
      @current-change="load"
      @size-change="search"
    />
  </el-card>
</template>

<style scoped>
.filter { margin-bottom: 4px; }
.tip { margin: 0 0 12px; font-size: 13px; color: #8A9099; }
.who { display: flex; flex-direction: column; line-height: 1.4; }
.who b { color: #262A31; font-weight: 600; }
.who small { color: #8A9099; }
.plus { color: #0E9F78; }
.minus { color: #FF4D3D; }
.pager { margin-top: 16px; justify-content: flex-end; }
</style>
