<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { errorMessage, type AdminSchemas } from '@xzji/api-client/admin'
import { api, formatTime } from '../api'
import { yuan } from '../content'
import { orderStatus, orderType, payMethodName } from '../trade'

type Order = AdminSchemas['AdminOrder']
type Detail = AdminSchemas['AdminOrderDetail']
type Status = AdminSchemas['OrderStatus']
type OType = AdminSchemas['OrderType']
type Method = 'wechat' | 'alipay' | 'balance'

const route = useRoute()
const filter = reactive<{ orderNo: string, email: string, userId?: number, type: OType | '', status: Status | '', payMethod: Method | '', duplicate: boolean }>({
  orderNo: '', email: '', userId: Number(route.query.userId) || undefined, type: '', status: '', payMethod: '', duplicate: false,
})
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const rows = ref<Order[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  const { data, error } = await api.GET('/orders', {
    params: {
      query: {
        orderNo: filter.orderNo.trim() || undefined, email: filter.email.trim() || undefined, userId: filter.userId,
        type: filter.type || undefined, status: filter.status || undefined, payMethod: filter.payMethod || undefined,
        duplicate: filter.duplicate || undefined, page: page.value, pageSize: pageSize.value,
      },
    },
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

function clearUser() {
  filter.userId = undefined
  search()
}

// ---------- 详情 ----------
const drawer = ref(false)
const detail = ref<Detail | null>(null)
const acting = ref(false)

async function openDetail(row: Order) {
  drawer.value = true
  detail.value = null
  const { data, error } = await api.GET('/orders/{orderNo}', { params: { path: { orderNo: row.orderNo } } })
  if (!data) {
    drawer.value = false
    return ElMessage.error(errorMessage(error))
  }
  detail.value = data
}

async function refreshDetail(o: Order) {
  const i = rows.value.findIndex(r => r.orderNo === o.orderNo)
  if (i >= 0) rows.value[i] = o
  await openDetail(o)
}

async function sync() {
  const no = detail.value!.order.orderNo
  acting.value = true
  const { data, error } = await api.POST('/orders/{orderNo}/sync', { params: { path: { orderNo: no } } })
  acting.value = false
  if (!data) return ElMessage.error(errorMessage(error))
  ElMessage.success(data.status === 'paid' ? '渠道已到账，订单已确认支付' : '渠道还没有收到付款')
  refreshDetail(data)
}

async function close() {
  const no = detail.value!.order.orderNo
  try {
    await ElMessageBox.confirm('关闭前会先查询一次支付渠道，已到账的会改为确认支付。确定关闭吗？', '关闭订单', { type: 'warning', confirmButtonText: '关闭订单', cancelButtonText: '取消' })
  } catch {
    return
  }
  acting.value = true
  const { data, error } = await api.POST('/orders/{orderNo}/close', { params: { path: { orderNo: no } } })
  acting.value = false
  if (!data) return ElMessage.error(errorMessage(error))
  ElMessage.success(data.status === 'paid' ? '渠道已到账，订单改为已支付' : '订单已关闭')
  refreshDetail(data)
}

// ---------- 退款 ----------
const refundDialog = ref(false)
const refundForm = reactive<{ method: 'balance' | 'original', remark: string }>({ method: 'balance', remark: '' })

function openRefund() {
  const o = detail.value!.order
  refundForm.method = o.type === 'recharge' ? 'original' : 'balance'
  refundForm.remark = ''
  refundDialog.value = true
}

async function refund() {
  const no = detail.value!.order.orderNo
  acting.value = true
  const { data, error } = await api.POST('/orders/{orderNo}/refund', { params: { path: { orderNo: no } }, body: refundForm })
  acting.value = false
  if (!data) return ElMessage.error(errorMessage(error))
  refundDialog.value = false
  ElMessage.success('已退款')
  refreshDetail(data)
}

const pretty = (v: unknown) => JSON.stringify(v, null, 2)

onMounted(load)
</script>

<template>
  <el-card shadow="never">
    <el-form inline class="filter" @submit.prevent="search">
      <el-form-item>
        <el-input v-model="filter.orderNo" placeholder="订单号" clearable style="width: 200px" @clear="search" />
      </el-form-item>
      <el-form-item>
        <el-input v-model="filter.email" placeholder="用户邮箱" clearable style="width: 200px" @clear="search" />
      </el-form-item>
      <el-form-item>
        <el-select v-model="filter.type" placeholder="全部类型" clearable style="width: 120px" @change="search">
          <el-option v-for="(label, v) in orderType" :key="v" :label="label" :value="v" />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-select v-model="filter.status" placeholder="全部状态" clearable style="width: 120px" @change="search">
          <el-option v-for="(s, v) in orderStatus" :key="v" :label="s.text" :value="v" />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-select v-model="filter.payMethod" placeholder="支付方式" clearable style="width: 120px" @change="search">
          <el-option v-for="(label, v) in payMethodName" :key="v" :label="label" :value="v" />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-checkbox v-model="filter.duplicate" @change="search">只看重复支付</el-checkbox>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" native-type="submit">搜索</el-button>
      </el-form-item>
    </el-form>
    <p v-if="filter.userId" class="tip">只显示用户 ID {{ filter.userId }} 的订单，<el-button link type="primary" @click="clearUser">查看全部</el-button></p>

    <el-table v-loading="loading" :data="rows" border stripe empty-text="没有找到符合条件的订单" @row-click="openDetail">
      <el-table-column prop="orderNo" label="订单号" width="200" />
      <el-table-column label="用户" min-width="180">
        <template #default="{ row }">
          <div class="who"><b>{{ row.user.nickname }}</b><small>{{ row.user.email }}</small></div>
        </template>
      </el-table-column>
      <el-table-column label="商品" min-width="200">
        <template #default="{ row }">
          <el-tag size="small" effect="plain">{{ orderType[row.type as OType] }}</el-tag> {{ row.title }}
          <el-tag v-if="row.grantError" size="small" type="danger">权益未发放</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="应付" width="110">
        <template #default="{ row }">
          <b>¥{{ yuan(row.payAmount) }}</b>
          <small v-if="row.discountAmount" class="exp">原价 ¥{{ yuan(row.amount) }}</small>
        </template>
      </el-table-column>
      <el-table-column label="支付方式" width="100">
        <template #default="{ row }">{{ payMethodName[row.payMethod as Method] ?? '-' }}</template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="orderStatus[row.status as Status].type">{{ orderStatus[row.status as Status].text }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="下单时间" width="150">
        <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="支付时间" width="150">
        <template #default="{ row }">{{ formatTime(row.paidAt) }}</template>
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

  <el-drawer v-model="drawer" title="订单详情" size="560px">
    <div v-if="detail" class="detail">
      <el-alert v-if="detail.order.grantError" type="error" :closable="false" show-icon :title="`付款已收到，但权益没有发放：${detail.order.grantError}`" description="请核实后手动开通（如在用户管理里设置会员），或给用户退款。" />
      <el-alert v-if="detail.payments.some(p => p.status === 'duplicate')" type="warning" :closable="false" show-icon title="这个订单收到了重复支付" description="标记为“重复”的那笔交易没有入账，需要到对应的支付平台原路退款。" />
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item label="订单号">{{ detail.order.orderNo }}</el-descriptions-item>
        <el-descriptions-item label="用户">{{ detail.order.user.nickname }}（{{ detail.order.user.email }}，ID {{ detail.order.user.id }}）</el-descriptions-item>
        <el-descriptions-item label="商品">{{ detail.order.title }}（{{ orderType[detail.order.type] }}，ID {{ detail.order.targetId }}）</el-descriptions-item>
        <el-descriptions-item label="金额">原价 ¥{{ yuan(detail.order.amount) }}，优惠 ¥{{ yuan(detail.order.discountAmount) }}，应付 <b>¥{{ yuan(detail.order.payAmount) }}</b></el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="orderStatus[detail.order.status].type">{{ orderStatus[detail.order.status].text }}</el-tag>
          <span v-if="detail.order.status === 'pending'" class="exp-inline">{{ formatTime(detail.order.expiresAt) }} 自动关闭</span>
        </el-descriptions-item>
        <el-descriptions-item label="支付方式">{{ payMethodName[detail.order.payMethod as Method] ?? '还没有发起支付' }}</el-descriptions-item>
        <el-descriptions-item label="下单时间">{{ formatTime(detail.order.createdAt) }}</el-descriptions-item>
        <el-descriptions-item label="支付时间">{{ formatTime(detail.order.paidAt) }}</el-descriptions-item>
        <el-descriptions-item v-if="detail.order.status === 'refunded'" label="退款">
          {{ detail.order.refundMethod === 'balance' ? '退回余额' : '原路退回' }}，{{ formatTime(detail.order.refundedAt) }}<template v-if="detail.order.refundRemark">，{{ detail.order.refundRemark }}</template>
        </el-descriptions-item>
      </el-descriptions>

      <div class="acts">
        <el-button v-if="detail.order.status === 'pending' && detail.order.payMethod && detail.order.payMethod !== 'balance'" :loading="acting" @click="sync">查询渠道补单</el-button>
        <el-button v-if="detail.order.status === 'pending'" :loading="acting" @click="close">关闭订单</el-button>
        <el-button v-if="detail.order.status === 'paid'" type="danger" plain :loading="acting" @click="openRefund">退款</el-button>
      </div>

      <h4>支付记录</h4>
      <el-table :data="detail.payments" border size="small" empty-text="没有第三方支付记录">
        <el-table-column label="渠道" width="80">
          <template #default="{ row }">{{ payMethodName[row.channel as Method] }}</template>
        </el-table-column>
        <el-table-column prop="tradeNo" label="交易号" min-width="160" />
        <el-table-column label="金额" width="90">
          <template #default="{ row }">¥{{ yuan(row.amount) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.status === 'success' ? 'success' : 'warning'" size="small">{{ row.status === 'success' ? '已入账' : '重复' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column type="expand">
          <template #default="{ row }"><pre class="raw">{{ row.raw }}</pre></template>
        </el-table-column>
      </el-table>

      <h4>下单快照与发放结果</h4>
      <pre class="raw">{{ pretty(detail.snapshot) }}</pre>
    </div>
    <div v-else v-loading="true" class="placeholder" />
  </el-drawer>

  <el-dialog v-model="refundDialog" title="退款" width="460px" destroy-on-close>
    <template v-if="detail">
      <p class="who-line">{{ detail.order.title }}，应付 ¥{{ yuan(detail.order.payAmount) }}</p>
      <el-form label-width="80px">
        <el-form-item label="退款方式">
          <el-radio-group v-model="refundForm.method">
            <el-radio value="balance" :disabled="detail.order.type === 'recharge'">退回余额</el-radio>
            <el-radio value="original" :disabled="detail.order.payMethod === 'balance'">已原路退回</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="refundForm.remark" maxlength="100" placeholder="如：用户误购（可以不填）" />
        </el-form-item>
      </el-form>
      <ul class="notes">
        <li v-if="refundForm.method === 'original'">“已原路退回”只做记录，请先在支付平台完成退款。</li>
        <li v-if="detail.order.type === 'recharge'">会同时从用户余额里扣回 ¥{{ yuan(detail.order.payAmount) }}，余额不足时无法退款。</li>
        <li v-if="detail.order.type === 'post'">退款后收回这篇内容的购买权益。</li>
        <li v-if="detail.order.type === 'membership'">会员时长不会自动扣减，需要的话在用户管理里调整。</li>
      </ul>
    </template>
    <template #footer>
      <el-button @click="refundDialog = false">取消</el-button>
      <el-button type="danger" :loading="acting" @click="refund">确认退款</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.filter { margin-bottom: 4px; }
.tip { margin: 0 0 12px; font-size: 13px; color: #8A9099; }
.who { display: flex; flex-direction: column; line-height: 1.4; }
.who b { color: #262A31; font-weight: 600; }
.who small, .exp { display: block; font-size: 12px; color: #8A9099; }
.exp-inline { margin-left: 8px; font-size: 12px; color: #8A9099; }
.pager { margin-top: 16px; justify-content: flex-end; }
.detail { display: flex; flex-direction: column; gap: 14px; }
.detail h4 { margin: 8px 0 0; font-size: 14px; color: #262A31; }
.acts { display: flex; gap: 8px; }
.raw { margin: 0; max-height: 320px; overflow: auto; padding: 10px 12px; border-radius: 6px; background: #F6F7F9; font-size: 12px; line-height: 1.6; white-space: pre-wrap; word-break: break-all; }
.placeholder { height: 200px; }
.who-line { margin: 0 0 12px; color: #262A31; }
.notes { margin: 0; padding-left: 18px; font-size: 12.5px; color: #8A9099; line-height: 1.8; }
:deep(.el-table__row) { cursor: pointer; }
</style>
