<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { errorMessage, type AdminSchemas } from '@xzji/api-client/admin'
import { api, formatTime } from '../api'

type Comment = AdminSchemas['AdminComment']
type Status = AdminSchemas['CommentStatus']
type Moderation = AdminSchemas['CommentSettings']['moderation']

const status = ref<Status | ''>('pending')
const keyword = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const rows = ref<Comment[]>([])
const loading = ref(false)
const moderation = ref<Moderation>('all')

const statusText: Record<Status, { text: string, type: 'warning' | 'success' | 'info' }> = {
  pending: { text: '待审核', type: 'warning' },
  approved: { text: '已通过', type: 'success' },
  spam: { text: '垃圾评论', type: 'info' },
}

async function load() {
  loading.value = true
  const { data, error } = await api.GET('/comments', {
    params: { query: { status: status.value || undefined, keyword: keyword.value || undefined, page: page.value, pageSize: pageSize.value } },
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

async function setStatus(row: Comment, s: Status) {
  const { data, error } = await api.PATCH('/comments/{id}', { params: { path: { id: row.id } }, body: { status: s } })
  if (!data) return ElMessage.error(errorMessage(error))
  ElMessage.success(s === 'approved' ? '已通过' : s === 'spam' ? '已标为垃圾评论' : '已改为待审核')
  load()
}

async function remove(row: Comment) {
  const tip = row.parentId ? '确定删除这条回复吗？' : '确定删除这条评论吗？它下面的回复也会一起删除。'
  try {
    await ElMessageBox.confirm(tip, '删除评论', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
  } catch {
    return
  }
  const { error, response } = await api.DELETE('/comments/{id}', { params: { path: { id: row.id } } })
  if (!response.ok) return ElMessage.error(errorMessage(error))
  ElMessage.success('已删除')
  load()
}

async function saveModeration(m: Moderation) {
  const { data, error } = await api.PUT('/settings/comment', { body: { moderation: m } })
  if (!data) {
    ElMessage.error(errorMessage(error))
    return loadSettings()
  }
  ElMessage.success('审核方式已保存')
}

async function loadSettings() {
  const { data } = await api.GET('/settings/comment')
  if (data) moderation.value = data.moderation
}

onMounted(() => {
  loadSettings()
  load()
})
</script>

<template>
  <el-card shadow="never">
    <div class="settings">
      <span class="label">审核方式</span>
      <el-radio-group v-model="moderation" @change="saveModeration">
        <el-radio value="all">全部先审核</el-radio>
        <el-radio value="trusted">有过通过评论的用户免审</el-radio>
        <el-radio value="none">不审核</el-radio>
      </el-radio-group>
      <span class="tip">“评论可见”的内容在评论通过后才解锁。管理员的评论始终直接通过。</span>
    </div>
    <div class="toolbar">
      <el-radio-group v-model="status" @change="search">
        <el-radio-button value="pending">待审核</el-radio-button>
        <el-radio-button value="approved">已通过</el-radio-button>
        <el-radio-button value="spam">垃圾评论</el-radio-button>
        <el-radio-button value="">全部</el-radio-button>
      </el-radio-group>
      <el-form inline @submit.prevent="search">
        <el-form-item>
          <el-input v-model="keyword" placeholder="搜索评论内容" clearable style="width: 220px" @clear="search" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" native-type="submit">搜索</el-button>
        </el-form-item>
      </el-form>
    </div>
    <el-table v-loading="loading" :data="rows" border :empty-text="status === 'pending' ? '没有待审核的评论' : '没有评论'">
      <el-table-column label="评论" min-width="320">
        <template #default="{ row }">
          <p class="content"><span v-if="row.replyTo" class="at">@{{ row.replyTo.nickname }}</span>{{ row.content }}</p>
          <small class="meta">{{ row.author.nickname }} · {{ formatTime(row.createdAt) }} · {{ row.ip || '-' }}</small>
        </template>
      </el-table-column>
      <el-table-column label="所在内容" min-width="200">
        <template #default="{ row }">
          <RouterLink :to="{ name: 'post-edit', params: { id: row.postId } }" class="post">{{ row.postTitle || `内容 ${row.postId}` }}</RouterLink>
          <small v-if="row.parentId" class="meta">回复</small>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="statusText[row.status as Status].type">{{ statusText[row.status as Status].text }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="190" fixed="right">
        <template #default="{ row }">
          <el-button v-if="row.status !== 'approved'" link type="primary" @click="setStatus(row, 'approved')">通过</el-button>
          <el-button v-if="row.status !== 'spam'" link type="warning" @click="setStatus(row, 'spam')">垃圾</el-button>
          <el-button v-if="row.status === 'approved'" link @click="setStatus(row, 'pending')">撤回</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
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
.settings { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; padding-bottom: 14px; margin-bottom: 14px; border-bottom: 1px solid #EEF0F3; }
.label { font-size: 14px; font-weight: 600; color: #262A31; }
.tip { font-size: 12px; color: #8A9099; }
.toolbar { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.content { margin: 0; color: #262A31; white-space: pre-wrap; word-break: break-word; }
.at { margin-right: 4px; color: #2F6BFF; }
.meta { display: block; font-size: 12px; color: #8A9099; }
.post { color: #262A31; text-decoration: none; }
.post:hover { color: #2F6BFF; }
.pager { margin-top: 16px; justify-content: flex-end; }
</style>
