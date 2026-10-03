<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { errorMessage, type AdminSchemas } from '@xzji/api-client/admin'
import { api, formatTime } from '../api'
import { me } from '../auth'

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

onMounted(load)
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
      <el-table-column label="邀请人" width="90">
        <template #default="{ row }">{{ row.inviterId ?? '-' }}</template>
      </el-table-column>
      <el-table-column label="最近登录" width="150">
        <template #default="{ row }">{{ formatTime(row.lastLoginAt) }}</template>
      </el-table-column>
      <el-table-column label="注册时间" width="150">
        <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="190" fixed="right">
        <template #default="{ row }">
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
</template>

<style scoped>
.filter { margin-bottom: 4px; }
.who { display: flex; flex-direction: column; line-height: 1.4; }
.who b { color: #262A31; font-weight: 600; }
.who small { color: #8A9099; }
.self { font-size: 12px; color: #8A9099; }
.pager { margin-top: 16px; justify-content: flex-end; }
</style>
