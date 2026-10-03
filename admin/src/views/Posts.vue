<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { errorMessage, type AdminSchemas } from '@xzji/api-client/admin'
import { api, formatTime } from '../api'
import { buildTree, categories, loadCategories, typeLabel } from '../content'

type Item = AdminSchemas['AdminPostItem']
type Status = AdminSchemas['PostStatus']
type PostType = AdminSchemas['PostType']

const router = useRouter()
const filter = reactive<{ type: PostType | '', status: Status | '', categoryId: number | null, keyword: string }>({
  type: '', status: '', categoryId: null, keyword: '',
})
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const rows = ref<Item[]>([])
const loading = ref(false)
const tree = computed(() => buildTree(categories.value))
const inTrash = computed(() => filter.status === 'trash')

async function load() {
  loading.value = true
  const { data, error } = await api.GET('/posts', {
    params: {
      query: {
        type: filter.type || undefined,
        status: filter.status || undefined,
        categoryId: filter.categoryId ?? undefined,
        keyword: filter.keyword || undefined,
        page: page.value,
        pageSize: pageSize.value,
      },
    },
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

// 已发布但发布时间未到的显示为“定时”
function statusOf(row: Item): { text: string, type: 'success' | 'info' | 'warning' | 'danger' } {
  if (row.status === 'trash') return { text: '回收站', type: 'danger' }
  if (row.status === 'draft') return { text: '草稿', type: 'info' }
  if (row.publishedAt && new Date(row.publishedAt) > new Date()) return { text: '定时', type: 'warning' }
  return { text: '已发布', type: 'success' }
}

async function confirm(text: string, title: string, button: string) {
  try {
    await ElMessageBox.confirm(text, title, { type: 'warning', confirmButtonText: button, cancelButtonText: '取消' })
    return true
  } catch {
    return false
  }
}

async function trash(row: Item) {
  if (!(await confirm(`确定把“${row.title}”移入回收站吗？前台将不再显示。`, '移入回收站', '移入回收站'))) return
  const { error, response } = await api.DELETE('/posts/{id}', { params: { path: { id: row.id } } })
  if (!response.ok) return ElMessage.error(errorMessage(error))
  ElMessage.success('已移入回收站')
  load()
}

async function restore(row: Item) {
  const { error, response } = await api.POST('/posts/{id}/restore', { params: { path: { id: row.id } } })
  if (!response.ok) return ElMessage.error(errorMessage(error))
  ElMessage.success('已恢复为草稿')
  load()
}

async function destroy(row: Item) {
  if (!(await confirm(`彻底删除“${row.title}”后无法恢复，确定吗？`, '彻底删除', '彻底删除'))) return
  const { error, response } = await api.DELETE('/posts/{id}', { params: { path: { id: row.id }, query: { permanent: true } } })
  if (!response.ok) return ElMessage.error(errorMessage(error))
  ElMessage.success('已彻底删除')
  load()
}

onMounted(() => {
  loadCategories()
  load()
})
</script>

<template>
  <el-card shadow="never">
    <div class="toolbar">
      <el-radio-group v-model="filter.status" @change="search">
        <el-radio-button value="">全部</el-radio-button>
        <el-radio-button value="published">已发布</el-radio-button>
        <el-radio-button value="draft">草稿</el-radio-button>
        <el-radio-button value="trash">回收站</el-radio-button>
      </el-radio-group>
      <div class="new">
        <el-button type="primary" @click="router.push({ name: 'post-new', query: { type: 'resource' } })">发布资源</el-button>
        <el-button @click="router.push({ name: 'post-new', query: { type: 'article' } })">写文章</el-button>
      </div>
    </div>
    <el-form inline class="filter" @submit.prevent="search">
      <el-form-item>
        <el-select v-model="filter.type" placeholder="全部类型" clearable style="width: 120px" @change="search">
          <el-option label="资源" value="resource" />
          <el-option label="文章" value="article" />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-tree-select
          v-model="filter.categoryId"
          :data="tree"
          :props="{ label: 'name', children: 'children' }"
          node-key="id"
          check-strictly
          default-expand-all
          placeholder="全部分类"
          clearable
          style="width: 180px"
          @change="search"
        />
      </el-form-item>
      <el-form-item>
        <el-input v-model="filter.keyword" placeholder="搜索标题" clearable style="width: 220px" @clear="search" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" native-type="submit">搜索</el-button>
      </el-form-item>
    </el-form>

    <el-table v-loading="loading" :data="rows" border stripe :empty-text="inTrash ? '回收站是空的' : '还没有内容'">
      <el-table-column label="标题" min-width="320">
        <template #default="{ row }">
          <div class="title">
            <img v-if="row.cover" :src="row.cover.url" alt="" class="thumb">
            <span v-else class="thumb none">{{ typeLabel[row.type as PostType] }}</span>
            <div>
              <RouterLink v-if="!inTrash" :to="{ name: 'post-edit', params: { id: row.id } }" class="link">{{ row.title }}</RouterLink>
              <span v-else>{{ row.title }}</span>
              <small><el-tag v-if="row.pinned" size="small" type="danger" effect="plain">置顶</el-tag> ID {{ row.id }} · {{ row.author.nickname }}</small>
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="类型" width="80">
        <template #default="{ row }">{{ typeLabel[row.type as PostType] }}</template>
      </el-table-column>
      <el-table-column label="分类" width="120">
        <template #default="{ row }">{{ row.category.name }}</template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="statusOf(row).type">{{ statusOf(row).text }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="浏览" width="80" prop="viewCount" />
      <el-table-column label="发布时间" width="150">
        <template #default="{ row }">{{ formatTime(row.publishedAt) }}</template>
      </el-table-column>
      <el-table-column label="更新时间" width="150">
        <template #default="{ row }">{{ formatTime(row.updatedAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <template v-if="row.status === 'trash'">
            <el-button link type="primary" @click="restore(row)">恢复</el-button>
            <el-button link type="danger" @click="destroy(row)">彻底删除</el-button>
          </template>
          <template v-else>
            <el-button link type="primary" @click="router.push({ name: 'post-edit', params: { id: row.id } })">编辑</el-button>
            <el-button link type="danger" @click="trash(row)">移入回收站</el-button>
          </template>
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
.filter { margin-bottom: 4px; }
.title { display: flex; align-items: center; gap: 10px; }
.thumb { flex: none; width: 64px; height: 40px; border-radius: 4px; object-fit: cover; background: #F6F7F9; }
.thumb.none { display: grid; place-items: center; font-size: 12px; color: #8A9099; }
.title div { display: flex; flex-direction: column; min-width: 0; line-height: 1.5; }
.title small { font-size: 12px; color: #8A9099; }
.link { color: #262A31; font-weight: 600; text-decoration: none; }
.link:hover { color: #2F6BFF; }
.pager { margin-top: 16px; justify-content: flex-end; }
</style>
