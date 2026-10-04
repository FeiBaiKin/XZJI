<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { errorMessage, type AdminSchemas } from '@xzji/api-client/admin'
import { api, formatTime } from '../api'
import { me } from '../auth'
import { levels, loadLevels } from '../content'

type UserItem = AdminSchemas['UserItem']
type Role = AdminSchemas['Role']
type Status = AdminSchemas['UserStatus']

const filter = reactive<{ keyword: string, role: Role | '', status: Status | '' }>({ keyword: '', role: '', status: '' })
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const rows = ref<UserItem[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const { data, error } = await api.GET('/users', {
      params: {
        query: {
          keyword: filter.keyword || undefined,
          role: filter.role || undefined,
          status: filter.status || undefined,
          page: page.value,
          pageSize: pageSize.value,
        },
      },
    })
    if (!data) {
      ElMessage.error(errorMessage(error))
      return
    }
    rows.value = data.items
    total.value = data.total
  } catch {
    ElMessage.error(errorMessage(null))
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

async function update(row: UserItem, change: { role?: Role, status?: Status }, confirmText: string) {
  try {
    await ElMessageBox.confirm(confirmText, '确认操作', { type: 'warning', confirmButtonText: '确定', cancelButtonText: '取消' })
  } catch {
    return
  }
  const { data, error } = await api.PATCH('/users/{id}', { params: { path: { id: row.id } }, body: change })
  if (!data) {
    ElMessage.error(errorMessage(error))
    return
  }
  Object.assign(row, data)
  ElMessage.success('已保存')
}

const toggleBan = (row: UserItem) => row.status === 'banned'
  ? update(row, { status: 'active' }, `确定解封 ${row.email} 吗？`)
  : update(row, { status: 'banned' }, `确定封禁 ${row.email} 吗？封禁后该用户会立即退出登录，并且无法再登录。`)

const toggleAdmin = (row: UserItem) => row.role === 'admin'
  ? update(row, { role: 'user' }, `确定取消 ${row.email} 的管理员身份吗？`)
  : update(row, { role: 'admin' }, `确定把 ${row.email} 设为管理员吗？管理员可以登录后台并管理所有用户。`)

// 设置会员：后台手动开通，不产生订单
const memberDialog = ref(false)
const memberRow = ref<UserItem | null>(null)
const memberForm = reactive<{ levelId: number | null, permanent: boolean, expiresAt: string | null }>({ levelId: null, permanent: true, expiresAt: null })
const memberSaving = ref(false)

function openMember(row: UserItem) {
  memberRow.value = row
  memberForm.levelId = row.membership?.levelId ?? levels.value[0]?.id ?? null
  memberForm.permanent = !row.membership?.expiresAt
  memberForm.expiresAt = row.membership?.expiresAt ?? null
  memberDialog.value = true
}

async function saveMember() {
  const row = memberRow.value
  if (!row || !memberForm.levelId) return ElMessage.warning('请选择会员等级')
  if (!memberForm.permanent && !memberForm.expiresAt) return ElMessage.warning('请选择到期时间')
  memberSaving.value = true
  const { data, error } = await api.PUT('/users/{id}/membership', {
    params: { path: { id: row.id } },
    body: { levelId: memberForm.levelId, expiresAt: memberForm.permanent ? null : memberForm.expiresAt },
  })
  memberSaving.value = false
  if (!data) return ElMessage.error(errorMessage(error))
  Object.assign(row, data)
  memberDialog.value = false
  ElMessage.success('会员已开通')
}

async function revokeMember() {
  const row = memberRow.value
  if (!row) return
  try {
    await ElMessageBox.confirm(`确定取消 ${row.email} 的会员吗？`, '取消会员', { type: 'warning', confirmButtonText: '确定', cancelButtonText: '取消' })
  } catch {
    return
  }
  const { error, response } = await api.DELETE('/users/{id}/membership', { params: { path: { id: row.id } } })
  if (!response.ok) return ElMessage.error(errorMessage(error))
  row.membership = undefined
  memberDialog.value = false
  ElMessage.success('会员已取消')
}

onMounted(() => {
  loadLevels()
  load()
})
</script>

<template>
  <el-card shadow="never">
    <el-form inline class="filter" @submit.prevent="search">
      <el-form-item>
        <el-input v-model="filter.keyword" placeholder="邮箱或昵称" clearable style="width: 220px" @clear="search" />
      </el-form-item>
      <el-form-item>
        <el-select v-model="filter.role" placeholder="全部角色" clearable style="width: 130px" @change="search">
          <el-option label="普通用户" value="user" />
          <el-option label="管理员" value="admin" />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-select v-model="filter.status" placeholder="全部状态" clearable style="width: 130px" @change="search">
          <el-option label="正常" value="active" />
          <el-option label="已封禁" value="banned" />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" native-type="submit">搜索</el-button>
      </el-form-item>
    </el-form>

    <el-table v-loading="loading" :data="rows" border stripe empty-text="没有找到符合条件的用户">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column label="用户" min-width="220">
        <template #default="{ row }">
          <div class="who"><b>{{ row.nickname }}</b><small>{{ row.email }}</small></div>
        </template>
      </el-table-column>
      <el-table-column label="角色" width="100">
        <template #default="{ row }">
          <el-tag :type="row.role === 'admin' ? 'warning' : 'info'" effect="plain">{{ row.role === 'admin' ? '管理员' : '普通用户' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.status === 'banned' ? 'danger' : 'success'">{{ row.status === 'banned' ? '已封禁' : '正常' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="会员" width="150">
        <template #default="{ row }">
          <template v-if="row.membership">
            <el-tag :type="row.membership.active ? 'warning' : 'info'" effect="dark" size="small">{{ row.membership.name }}</el-tag>
            <small class="exp">{{ !row.membership.active ? '已过期' : row.membership.expiresAt ? `${formatTime(row.membership.expiresAt).slice(0, 10)} 到期` : '永久' }}</small>
          </template>
          <span v-else class="self">-</span>
        </template>
      </el-table-column>
      <el-table-column label="邀请人" width="90">
        <template #default="{ row }">{{ row.inviterId ?? '-' }}</template>
      </el-table-column>
      <el-table-column label="最近登录" width="150">
        <template #default="{ row }">{{ formatTime(row.lastLoginAt) }}</template>
      </el-table-column>
      <el-table-column label="注册时间" width="150">
        <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="250" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openMember(row)">设置会员</el-button>
          <template v-if="row.id !== me?.id">
            <el-button link :type="row.status === 'banned' ? 'primary' : 'danger'" @click="toggleBan(row)">{{ row.status === 'banned' ? '解封' : '封禁' }}</el-button>
            <el-button link type="primary" @click="toggleAdmin(row)">{{ row.role === 'admin' ? '取消管理员' : '设为管理员' }}</el-button>
          </template>
          <span v-else class="self">当前账号</span>
        </template>
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

  <el-dialog v-model="memberDialog" title="设置会员" width="440px" destroy-on-close>
    <p class="who-line">{{ memberRow?.nickname }}（{{ memberRow?.email }}）</p>
    <el-form label-width="72px">
      <el-form-item label="等级">
        <el-select v-model="memberForm.levelId" placeholder="选择会员等级" style="width: 100%">
          <el-option v-for="l in levels" :key="l.id" :label="l.status === 'active' ? l.name : `${l.name}（已停用）`" :value="l.id" />
        </el-select>
        <p v-if="!levels.length" class="hint">还没有会员等级，<RouterLink :to="{ name: 'levels' }">先去新建</RouterLink>。</p>
      </el-form-item>
      <el-form-item label="有效期">
        <el-radio-group v-model="memberForm.permanent">
          <el-radio :value="true">永久</el-radio>
          <el-radio :value="false">到期时间</el-radio>
        </el-radio-group>
        <el-date-picker
          v-if="!memberForm.permanent"
          v-model="memberForm.expiresAt"
          type="datetime"
          value-format="YYYY-MM-DDTHH:mm:ssZ"
          placeholder="选择到期时间"
          style="width: 100%; margin-top: 8px"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button v-if="memberRow?.membership" type="danger" plain style="float: left" @click="revokeMember">取消会员</el-button>
      <el-button @click="memberDialog = false">关闭</el-button>
      <el-button type="primary" :loading="memberSaving" @click="saveMember">保存</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.filter { margin-bottom: 4px; }
.who { display: flex; flex-direction: column; line-height: 1.4; }
.who b { color: #262A31; font-weight: 600; }
.who small { color: #8A9099; }
.self { font-size: 12px; color: #8A9099; }
.pager { margin-top: 16px; justify-content: flex-end; }
.exp { display: block; font-size: 12px; color: #8A9099; }
.who-line { margin: 0 0 12px; color: #262A31; }
.hint { margin: 4px 0 0; font-size: 12px; color: #8A9099; }
</style>
