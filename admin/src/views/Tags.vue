<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { errorMessage, type AdminSchemas } from '@xzji/api-client/admin'
import { api, formatTime } from '../api'

type Tag = AdminSchemas['AdminTag']

const keyword = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const rows = ref<Tag[]>([])
const loading = ref(false)

const dialog = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)
const form = reactive({ name: '', slug: '' })

async function load() {
  loading.value = true
  const { data, error } = await api.GET('/tags', {
    params: { query: { keyword: keyword.value || undefined, page: page.value, pageSize: pageSize.value } },
  })
  loading.value = false
  if (!data) {
    ElMessage.error(errorMessage(error))
    return
  }
  rows.value = data.items
  total.value = data.total
}

function search() {
  page.value = 1
  load()
}

function open(row?: Tag) {
  editingId.value = row?.id ?? null
  form.name = row?.name ?? ''
  form.slug = row?.slug ?? ''
  dialog.value = true
}

async function save() {
  saving.value = true
  const body = { name: form.name, slug: form.slug || undefined }
  const { data, error } = editingId.value
    ? await api.PUT('/tags/{id}', { params: { path: { id: editingId.value } }, body })
    : await api.POST('/tags', { body })
  saving.value = false
  if (!data) {
    ElMessage.error(errorMessage(error))
    return
  }
  dialog.value = false
  ElMessage.success('已保存')
  load()
}

async function remove(row: Tag) {
  try {
    await ElMessageBox.confirm(`确定删除标签“${row.name}”吗？用到它的 ${row.postCount} 篇内容会去掉这个标签。`, '删除标签', {
      type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消',
    })
  } catch {
    return
  }
  const { error, response } = await api.DELETE('/tags/{id}', { params: { path: { id: row.id } } })
  if (!response.ok) {
    ElMessage.error(errorMessage(error))
    return
  }
  ElMessage.success('已删除')
  load()
}

onMounted(load)
</script>

<template>
  <el-card shadow="never">
    <div class="toolbar">
      <el-form inline @submit.prevent="search">
        <el-form-item>
          <el-input v-model="keyword" placeholder="名称或 slug" clearable style="width: 220px" @clear="search" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" native-type="submit">搜索</el-button>
        </el-form-item>
      </el-form>
      <el-button type="primary" @click="open()">新建标签</el-button>
    </div>
    <el-table v-loading="loading" :data="rows" border stripe empty-text="没有找到标签。编辑内容时输入新标签会自动创建。">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="名称" min-width="160" />
      <el-table-column label="网址" min-width="200">
        <template #default="{ row }"><code>/tag/{{ row.slug }}/</code></template>
      </el-table-column>
      <el-table-column prop="postCount" label="内容数" width="90" />
      <el-table-column label="创建时间" width="150">
        <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="130" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="open(row)">编辑</el-button>
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

  <el-dialog v-model="dialog" :title="editingId ? '编辑标签' : '新建标签'" width="440px" destroy-on-close>
    <el-form label-width="64px" @submit.prevent="save">
      <el-form-item label="名称" required>
        <el-input v-model="form.name" maxlength="32" />
      </el-form-item>
      <el-form-item label="Slug">
        <el-input v-model="form.slug" maxlength="64" placeholder="留空时由名称生成，可以是中文" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialog = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">保存</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.toolbar { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.pager { margin-top: 16px; justify-content: flex-end; }
code { font-size: 12px; color: #8A9099; }
</style>
