<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { errorMessage, type AdminSchemas } from '@xzji/api-client/admin'
import { api } from '../api'
import { levels, loadLevels, yuan, type Level } from '../content'

type Plan = AdminSchemas['MembershipPlan']
type LevelForm = Omit<Required<AdminSchemas['LevelInput']>, 'perks'> & { perksText: string }

const loading = ref(false)
const plans = ref<Plan[]>([])

async function reload() {
  loading.value = true
  const [, res] = await Promise.all([loadLevels(true), api.GET('/membership/plans')])
  plans.value = res.data?.items ?? []
  loading.value = false
}

// ---------- 等级 ----------
const levelDialog = ref(false)
const levelId = ref<number | null>(null)
const levelSaving = ref(false)
const emptyLevel = (): LevelForm => ({
  name: '', description: '', rank: (levels.value.at(-1)?.rank ?? 0) + 1, discountRate: 100, dailyDownloadLimit: 0, status: 'active', perksText: '',
})
const levelForm = reactive<LevelForm>(emptyLevel())

function openLevel(row?: Level) {
  levelId.value = row?.id ?? null
  Object.assign(levelForm, row
    ? { name: row.name, description: row.description, rank: row.rank, discountRate: row.discountRate, dailyDownloadLimit: row.dailyDownloadLimit, status: row.status, perksText: row.perks.join('\n') }
    : emptyLevel())
  levelDialog.value = true
}

async function saveLevel() {
  const { perksText, ...rest } = levelForm
  const body = { ...rest, perks: perksText.split('\n').map(s => s.trim()).filter(Boolean) }
  levelSaving.value = true
  const { data, error } = levelId.value
    ? await api.PUT('/membership/levels/{id}', { params: { path: { id: levelId.value } }, body })
    : await api.POST('/membership/levels', { body })
  levelSaving.value = false
  if (!data) return ElMessage.error(errorMessage(error))
  levelDialog.value = false
  ElMessage.success('已保存')
  reload()
}

async function removeLevel(row: Level) {
  try {
    await ElMessageBox.confirm(`确定删除等级“${row.name}”吗？`, '删除等级', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
  } catch {
    return
  }
  const { error, response } = await api.DELETE('/membership/levels/{id}', { params: { path: { id: row.id } } })
  if (!response.ok) return ElMessage.error(errorMessage(error))
  ElMessage.success('已删除')
  reload()
}

// ---------- 套餐 ----------
type PlanForm = Required<AdminSchemas['PlanInput']>
const planDialog = ref(false)
const planId = ref<number | null>(null)
const planSaving = ref(false)
// 价格在表单里按元填写，提交时换算成分
const planYuan = reactive({ price: 0, originalPrice: 0 })
const emptyPlan = (): PlanForm => ({ levelId: levels.value[0]?.id ?? 0, name: '', durationDays: 365, price: 0, originalPrice: 0, badge: '', status: 'active', sort: 0 })
const planForm = reactive<PlanForm>(emptyPlan())
const permanent = computed({
  get: () => planForm.durationDays === 0,
  set: (v: boolean) => { planForm.durationDays = v ? 0 : 365 },
})

const levelName = (id: number) => levels.value.find(l => l.id === id)?.name ?? '已删除的等级'
// 按等级数字、排序显示
const sortedPlans = computed(() => {
  const rank = (id: number) => levels.value.find(l => l.id === id)?.rank ?? 999
  return [...plans.value].sort((a, b) => rank(a.levelId) - rank(b.levelId) || a.sort - b.sort || a.id - b.id)
})

function openPlan(row?: Plan) {
  planId.value = row?.id ?? null
  Object.assign(planForm, row
    ? { levelId: row.levelId, name: row.name, durationDays: row.durationDays, price: row.price, originalPrice: row.originalPrice, badge: row.badge, status: row.status, sort: row.sort }
    : emptyPlan())
  planYuan.price = planForm.price / 100
  planYuan.originalPrice = planForm.originalPrice / 100
  planDialog.value = true
}

async function savePlan() {
  const body = { ...planForm, price: Math.round(planYuan.price * 100), originalPrice: Math.round(planYuan.originalPrice * 100) }
  planSaving.value = true
  const { data, error } = planId.value
    ? await api.PUT('/membership/plans/{id}', { params: { path: { id: planId.value } }, body })
    : await api.POST('/membership/plans', { body })
  planSaving.value = false
  if (!data) return ElMessage.error(errorMessage(error))
  planDialog.value = false
  ElMessage.success('已保存')
  reload()
}

async function removePlan(row: Plan) {
  try {
    await ElMessageBox.confirm(`确定删除套餐“${levelName(row.levelId)} ${row.name}”吗？已下单的订单不受影响。`, '删除套餐', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
  } catch {
    return
  }
  const { error, response } = await api.DELETE('/membership/plans/{id}', { params: { path: { id: row.id } } })
  if (!response.ok) return ElMessage.error(errorMessage(error))
  ElMessage.success('已删除')
  reload()
}

const durationLabel = (d: number) => d === 0 ? '永久' : d % 365 === 0 ? `${d / 365} 年（${d} 天）` : `${d} 天`

onMounted(reload)
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="head">
        <b>会员等级</b>
        <el-button type="primary" @click="openLevel()">新建等级</el-button>
      </div>
    </template>
    <p class="tip">等级数字越大越高。内容设置“VIP 及以上免费”时，按这个数字比较。前台会员页按折扣和每日下载次数自动生成权益说明，其他权益写在“自定义权益”里，高等级自动包含低等级的自定义权益。</p>
    <el-table v-loading="loading" :data="levels" border empty-text="还没有会员等级">
      <el-table-column prop="rank" label="等级数字" width="100" />
      <el-table-column label="名称" min-width="160">
        <template #default="{ row }">
          <div class="who"><b>{{ row.name }}</b><small v-if="row.description">{{ row.description }}</small></div>
        </template>
      </el-table-column>
      <el-table-column label="购买折扣" width="100">
        <template #default="{ row }">{{ row.discountRate === 100 ? '不打折' : `${row.discountRate / 10} 折` }}</template>
      </el-table-column>
      <el-table-column label="每日会员下载" width="120">
        <template #default="{ row }">{{ row.dailyDownloadLimit === 0 ? '不限' : `${row.dailyDownloadLimit} 次` }}</template>
      </el-table-column>
      <el-table-column label="自定义权益" min-width="180">
        <template #default="{ row }">
          <span v-if="!row.perks.length" class="muted">-</span>
          <el-tag v-for="p in row.perks" :key="p" size="small" effect="plain" class="perk">{{ p }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="memberCount" label="会员数" width="80" />
      <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' ? 'success' : 'info'">{{ row.status === 'active' ? '启用' : '停用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openLevel(row)">编辑</el-button>
          <el-button link type="danger" @click="removeLevel(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <el-card shadow="never" class="gap">
    <template #header>
      <div class="head">
        <b>售卖套餐</b>
        <el-button type="primary" :disabled="!levels.length" @click="openPlan()">新建套餐</el-button>
      </div>
    </template>
    <p class="tip">会员升级按剩余价值折算，日均价取该等级 365 天套餐（年卡）的价格 ÷ 365，没有年卡时取时长最长的有限期套餐。永久会员升级时补两者永久套餐的差价，所以每个等级只能有一个启用的永久套餐。</p>
    <el-table v-loading="loading" :data="sortedPlans" border empty-text="还没有套餐，前台会员页不会显示可购买的等级">
      <el-table-column label="等级" width="120">
        <template #default="{ row }">{{ levelName(row.levelId) }}</template>
      </el-table-column>
      <el-table-column label="套餐" min-width="140">
        <template #default="{ row }">{{ row.name }} <el-tag v-if="row.badge" size="small" type="danger" effect="plain">{{ row.badge }}</el-tag></template>
      </el-table-column>
      <el-table-column label="时长" width="140">
        <template #default="{ row }">{{ durationLabel(row.durationDays) }}</template>
      </el-table-column>
      <el-table-column label="售价" width="110">
        <template #default="{ row }">¥{{ yuan(row.price) }}</template>
      </el-table-column>
      <el-table-column label="划线价" width="110">
        <template #default="{ row }">{{ row.originalPrice ? `¥${yuan(row.originalPrice)}` : '-' }}</template>
      </el-table-column>
      <el-table-column prop="sort" label="排序" width="70" />
      <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' ? 'success' : 'info'">{{ row.status === 'active' ? '在售' : '下架' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openPlan(row)">编辑</el-button>
          <el-button link type="danger" @click="removePlan(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <el-dialog v-model="levelDialog" :title="levelId ? '编辑等级' : '新建等级'" width="500px" destroy-on-close>
    <el-form label-width="110px" @submit.prevent="saveLevel">
      <el-form-item label="名称" required>
        <el-input v-model="levelForm.name" maxlength="16" placeholder="如：VIP、SVIP" />
      </el-form-item>
      <el-form-item label="一句话介绍">
        <el-input v-model="levelForm.description" maxlength="32" placeholder="如：适合经常下载（可以不填）" />
      </el-form-item>
      <el-form-item label="等级数字" required>
        <el-input-number v-model="levelForm.rank" :min="1" :max="99" controls-position="right" />
      </el-form-item>
      <el-form-item label="购买折扣">
        <el-input-number v-model="levelForm.discountRate" :min="1" :max="100" controls-position="right" />
        <span class="unit">% （80 表示 8 折，100 不打折）</span>
      </el-form-item>
      <el-form-item label="每日会员下载">
        <el-input-number v-model="levelForm.dailyDownloadLimit" :min="0" :max="9999" controls-position="right" />
        <span class="unit">次（0 表示不限）</span>
      </el-form-item>
      <el-form-item label="自定义权益">
        <el-input v-model="levelForm.perksText" type="textarea" :rows="3" placeholder="每行一条，最多 8 条，如：失效链接优先补" />
      </el-form-item>
      <el-form-item label="状态">
        <el-radio-group v-model="levelForm.status">
          <el-radio value="active">启用</el-radio>
          <el-radio value="disabled">停用</el-radio>
        </el-radio-group>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="levelDialog = false">取消</el-button>
      <el-button type="primary" :loading="levelSaving" @click="saveLevel">保存</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="planDialog" :title="planId ? '编辑套餐' : '新建套餐'" width="500px" destroy-on-close>
    <el-form label-width="90px" @submit.prevent="savePlan">
      <el-form-item label="等级" required>
        <el-select v-model="planForm.levelId" style="width: 100%">
          <el-option v-for="l in levels" :key="l.id" :label="l.status === 'active' ? l.name : `${l.name}（已停用）`" :value="l.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="名称" required>
        <el-input v-model="planForm.name" maxlength="16" placeholder="如：月卡、年卡、永久" />
      </el-form-item>
      <el-form-item label="时长" required>
        <el-radio-group v-model="permanent">
          <el-radio :value="false">有限期</el-radio>
          <el-radio :value="true">永久</el-radio>
        </el-radio-group>
        <template v-if="!permanent">
          <el-input-number v-model="planForm.durationDays" :min="1" :max="3650" controls-position="right" class="days" />
          <span class="unit">天（年卡请填 365）</span>
        </template>
      </el-form-item>
      <el-form-item label="售价" required>
        <el-input-number v-model="planYuan.price" :min="0.01" :max="99999" :precision="2" :step="1" controls-position="right" />
        <span class="unit">元</span>
      </el-form-item>
      <el-form-item label="划线价">
        <el-input-number v-model="planYuan.originalPrice" :min="0" :max="99999" :precision="2" :step="1" controls-position="right" />
        <span class="unit">元，0 表示不显示</span>
      </el-form-item>
      <el-form-item label="角标">
        <el-input v-model="planForm.badge" maxlength="6" placeholder="如：最多人选（可以不填）" />
      </el-form-item>
      <el-form-item label="排序">
        <el-input-number v-model="planForm.sort" :min="0" :max="999" controls-position="right" />
        <span class="unit">越小越靠前</span>
      </el-form-item>
      <el-form-item label="状态">
        <el-radio-group v-model="planForm.status">
          <el-radio value="active">在售</el-radio>
          <el-radio value="disabled">下架</el-radio>
        </el-radio-group>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="planDialog = false">取消</el-button>
      <el-button type="primary" :loading="planSaving" @click="savePlan">保存</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.head { display: flex; align-items: center; justify-content: space-between; }
.head b { font-size: 15px; color: #262A31; }
.tip { margin: 0 0 14px; font-size: 13px; color: #8A9099; line-height: 1.7; }
.unit { margin-left: 8px; font-size: 12px; color: #8A9099; }
.gap { margin-top: 20px; }
.who { display: flex; flex-direction: column; line-height: 1.4; }
.who b { color: #262A31; font-weight: 600; }
.who small, .muted { color: #8A9099; }
.perk { margin: 2px 4px 2px 0; }
.days { margin-left: 12px; }
</style>
