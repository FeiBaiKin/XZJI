<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { errorMessage, type AdminSchemas } from '@xzji/api-client/admin'
import { api } from '../api'
import { levels, loadLevels, type Level } from '../content'

type Form = Required<AdminSchemas['LevelInput']>

const loading = ref(false)
const dialog = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)
const empty = (): Form => ({ name: '', rank: (levels.value.at(-1)?.rank ?? 0) + 1, discountRate: 100, dailyDownloadLimit: 0, status: 'active' })
const form = reactive<Form>(empty())

async function reload() {
  loading.value = true
  await loadLevels(true)
  loading.value = false
}

function open(row?: Level) {
  editingId.value = row?.id ?? null
  Object.assign(form, row ? { name: row.name, rank: row.rank, discountRate: row.discountRate, dailyDownloadLimit: row.dailyDownloadLimit, status: row.status } : empty())
  dialog.value = true
}

async function save() {
  saving.value = true
  const { data, error } = editingId.value
    ? await api.PUT('/membership/levels/{id}', { params: { path: { id: editingId.value } }, body: form })
    : await api.POST('/membership/levels', { body: form })
  saving.value = false
  if (!data) return ElMessage.error(errorMessage(error))
  dialog.value = false
  ElMessage.success('已保存')
  reload()
}

async function remove(row: Level) {
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

onMounted(reload)
</script>

<template>
  <el-card shadow="never">
    <div class="toolbar">
      <p class="tip">等级数字越大越高。内容设置“VIP 及以上免费”时，按这个数字比较。套餐售卖在支付功能上线后配置。</p>
      <el-button type="primary" @click="open()">新建等级</el-button>
    </div>
    <el-table v-loading="loading" :data="levels" border empty-text="还没有会员等级">
      <el-table-column prop="rank" label="等级数字" width="100" />
      <el-table-column prop="name" label="名称" min-width="140" />
      <el-table-column label="购买折扣" width="110">
        <template #default="{ row }">{{ row.discountRate === 100 ? '不打折' : `${row.discountRate / 10} 折` }}</template>
      </el-table-column>
      <el-table-column label="每日会员下载" width="130">
        <template #default="{ row }">{{ row.dailyDownloadLimit === 0 ? '不限' : `${row.dailyDownloadLimit} 次` }}</template>
      </el-table-column>
      <el-table-column prop="memberCount" label="会员数" width="90" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' ? 'success' : 'info'">{{ row.status === 'active' ? '启用' : '停用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="130" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="open(row)">编辑</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <el-dialog v-model="dialog" :title="editingId ? '编辑等级' : '新建等级'" width="460px" destroy-on-close>
    <el-form label-width="110px" @submit.prevent="save">
      <el-form-item label="名称" required>
        <el-input v-model="form.name" maxlength="16" placeholder="如：VIP、SVIP" />
      </el-form-item>
      <el-form-item label="等级数字" required>
        <el-input-number v-model="form.rank" :min="1" :max="99" controls-position="right" />
      </el-form-item>
      <el-form-item label="购买折扣">
        <el-input-number v-model="form.discountRate" :min="1" :max="100" controls-position="right" />
        <span class="unit">% （80 表示 8 折，100 不打折）</span>
      </el-form-item>
      <el-form-item label="每日会员下载">
        <el-input-number v-model="form.dailyDownloadLimit" :min="0" :max="9999" controls-position="right" />
        <span class="unit">次（0 表示不限）</span>
      </el-form-item>
      <el-form-item label="状态">
        <el-radio-group v-model="form.status">
          <el-radio value="active">启用</el-radio>
          <el-radio value="disabled">停用</el-radio>
        </el-radio-group>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialog = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">保存</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.toolbar { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 14px; }
.tip { margin: 0; font-size: 13px; color: #8A9099; }
.unit { margin-left: 8px; font-size: 12px; color: #8A9099; }
</style>
