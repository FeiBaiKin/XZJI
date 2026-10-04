<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { errorMessage, type AdminSchemas } from '@xzji/api-client/admin'
import { api, formatTime } from '../api'

type Report = AdminSchemas['LinkReport']
type Status = AdminSchemas['ReportStatus']

const status = ref<Status | ''>('pending')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const rows = ref<Report[]>([])
const loading = ref(false)

const statusText: Record<Status, { text: string, type: 'warning' | 'success' | 'info' }> = {
  pending: { text: '待处理', type: 'warning' },
  resolved: { text: '已补链接', type: 'success' },
  ignored: { text: '已忽略', type: 'info' },
}

async function load() {
  loading.value = true
  const { data, error } = await api.GET('/link-reports', {
    params: { query: { status: status.value || undefined, page: page.value, pageSize: pageSize.value } },
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

async function setStatus(row: Report, s: Status) {
  const { error, response } = await api.PATCH('/link-reports/{id}', { params: { path: { id: row.id } }, body: { status: s } })
  if (!response.ok) return ElMessage.error(errorMessage(error))
  ElMessage.success(statusText[s].text)
  load()
}

onMounted(load)
</script>

<template>
  <el-card shadow="never">
    <div class="toolbar">
      <el-radio-group v-model="status" @change="search">
        <el-radio-button value="pending">待处理</el-radio-button>
        <el-radio-button value="resolved">已补链接</el-radio-button>
        <el-radio-button value="ignored">已忽略</el-radio-button>
        <el-radio-button value="">全部</el-radio-button>
      </el-radio-group>
      <p class="tip">在内容编辑页更换下载地址后，回到这里标记“已补链接”。</p>
    </div>
    <el-table v-loading="loading" :data="rows" border :empty-text="status === 'pending' ? '没有待处理的反馈' : '没有反馈'">
      <el-table-column label="资源" min-width="220">
        <template #default="{ row }">
          <RouterLink :to="{ name: 'post-edit', params: { id: row.postId } }" class="post">{{ row.postTitle || `内容 ${row.postId}` }}</RouterLink>
        </template>
      </el-table-column>
      <el-table-column label="下载地址" min-width="240">
        <template #default="{ row }">
          <template v-if="row.linkName">
            <b class="name">{{ row.linkName }}</b>
            <el-tag v-if="row.linkStatus === 'broken'" size="small" type="danger" effect="plain">已标记失效</el-tag>
            <a :href="row.linkUrl" target="_blank" rel="noopener noreferrer" class="url">{{ row.linkUrl }}</a>
          </template>
          <span v-else class="gone">地址已被删除</span>
        </template>
      </el-table-column>
      <el-table-column label="反馈" min-width="200">
        <template #default="{ row }">
          <p class="reason">{{ row.reason || '（没有填写说明）' }}</p>
          <small class="meta">{{ row.user.nickname }} · {{ formatTime(row.createdAt) }}</small>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="statusText[row.status as Status].type">{{ statusText[row.status as Status].text }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="210" fixed="right">
        <template #default="{ row }">
          <RouterLink :to="{ name: 'post-edit', params: { id: row.postId } }" class="act">去修改</RouterLink>
          <el-button v-if="row.status !== 'resolved'" link type="primary" @click="setStatus(row, 'resolved')">已补链接</el-button>
          <el-button v-if="row.status === 'pending'" link @click="setStatus(row, 'ignored')">忽略</el-button>
          <el-button v-else link @click="setStatus(row, 'pending')">重新打开</el-button>
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
.toolbar { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 14px; }
.tip { margin: 0; font-size: 13px; color: #8A9099; }
.post, .act { color: #262A31; text-decoration: none; }
.post:hover, .act:hover { color: #2F6BFF; }
.act { margin-right: 12px; font-size: 14px; color: #2F6BFF; }
.name { margin-right: 6px; color: #262A31; }
.url { display: block; font-size: 12px; color: #8A9099; word-break: break-all; }
.gone { color: #8A9099; }
.reason { margin: 0; color: #262A31; }
.meta { font-size: 12px; color: #8A9099; }
.pager { margin-top: 16px; justify-content: flex-end; }
</style>
