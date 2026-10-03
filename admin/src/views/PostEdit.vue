<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { errorMessage, type AdminSchemas } from '@xzji/api-client/admin'
import { api, formatTime, type Media } from '../api'
import { buildTree, categories, loadCategories, typeLabel } from '../content'
import ImagePicker from '../components/ImagePicker.vue'
import RichEditor from '../components/RichEditor.vue'

type Post = AdminSchemas['AdminPost']
type PostType = AdminSchemas['PostType']
type Attr = AdminSchemas['Attr']
type Log = AdminSchemas['ChangelogEntry']

const defaultPlatforms = ['Windows', 'Android', 'macOS', 'Linux', 'iOS', '源码', '主题']

const route = useRoute()
const router = useRouter()

const postId = ref<number | null>(null)
const post = ref<Post | null>(null)
const loading = ref(false)
const saving = ref(false)

const empty = (type: PostType) => ({
  type,
  title: '',
  summary: '',
  contentHtml: '',
  cover: null as Media | null,
  categoryId: null as number | null,
  tags: [] as string[],
  pinned: false,
  publishedAt: null as string | null,
  seoTitle: '',
  seoKeywords: '',
  seoDescription: '',
  resource: { platform: '', version: '', fileSize: '', icon: null as Media | null, attrs: [] as Attr[], changelog: [] as Log[] },
})
const form = reactive(empty('resource'))
let snapshot = ''
const dirty = computed(() => JSON.stringify(form) !== snapshot)
const markClean = () => { snapshot = JSON.stringify(form) }

const tree = computed(() => buildTree(categories.value))
const published = computed(() => post.value?.status === 'published')
const scheduled = computed(() => !!form.publishedAt && new Date(form.publishedAt) > new Date())

function fill(p: Post) {
  post.value = p
  postId.value = p.id
  Object.assign(form, {
    type: p.type, title: p.title, summary: p.summary, contentHtml: p.contentHtml, cover: p.cover ?? null,
    categoryId: p.categoryId, tags: [...p.tags], pinned: p.pinned, publishedAt: p.publishedAt ?? null,
    seoTitle: p.seoTitle, seoKeywords: p.seoKeywords, seoDescription: p.seoDescription,
    resource: p.resource
      ? { ...p.resource, icon: p.resource.icon ?? null, attrs: p.resource.attrs.map(a => ({ ...a })), changelog: p.resource.changelog.map(l => ({ ...l })) }
      : empty(p.type).resource,
  })
  markClean()
}

async function load() {
  const id = Number(route.params.id)
  if (!id) {
    post.value = null
    postId.value = null
    Object.assign(form, empty(route.query.type === 'article' ? 'article' : 'resource'))
    markClean()
    return
  }
  if (id === postId.value) return
  loading.value = true
  const { data, error } = await api.GET('/posts/{id}', { params: { path: { id } } })
  loading.value = false
  if (!data) {
    ElMessage.error(errorMessage(error))
    return
  }
  fill(data)
}

async function save(status: 'draft' | 'published') {
  if (!form.title.trim()) return ElMessage.warning('请填写标题')
  if (!form.categoryId) return ElMessage.warning('请选择分类')
  const r = form.resource
  const body: AdminSchemas['PostInput'] = {
    type: form.type, title: form.title, summary: form.summary, contentHtml: form.contentHtml,
    coverId: form.cover?.id ?? null, categoryId: form.categoryId, tags: form.tags, status, pinned: form.pinned,
    publishedAt: form.publishedAt, seoTitle: form.seoTitle, seoKeywords: form.seoKeywords, seoDescription: form.seoDescription,
    resource: form.type === 'resource'
      ? { platform: r.platform, version: r.version, fileSize: r.fileSize, iconId: r.icon?.id ?? null, attrs: r.attrs, changelog: r.changelog }
      : undefined,
  }
  const wasPublished = published.value
  saving.value = true
  const { data, error } = postId.value
    ? await api.PUT('/posts/{id}', { params: { path: { id: postId.value } }, body })
    : await api.POST('/posts', { body })
  saving.value = false
  if (!data) {
    ElMessage.error(errorMessage(error))
    return
  }
  const created = !postId.value
  fill(data)
  ElMessage.success(status === 'draft' ? '草稿已保存' : wasPublished ? '已更新' : '已发布')
  if (created) await router.replace({ name: 'post-edit', params: { id: data.id } })
}

// 标签：输入时搜索已有标签，也可以直接输入新标签
const tagOptions = ref<string[]>([])
async function searchTags(keyword: string) {
  const { data } = await api.GET('/tags', { params: { query: { keyword: keyword || undefined, pageSize: 20 } } })
  tagOptions.value = data?.items.map(t => t.name) ?? []
}

const platforms = ref<string[]>(defaultPlatforms)
async function loadPlatforms() {
  const { data } = await api.GET('/platforms')
  if (data) platforms.value = [...new Set([...defaultPlatforms, ...data.items])]
}

const addAttr = () => form.resource.attrs.push({ name: '', value: '' })
const addLog = () => form.resource.changelog.unshift({ version: form.resource.version, date: new Date().toISOString().slice(0, 10), note: '' })

watch(() => [route.params.id, route.query.type], load)

onBeforeRouteLeave(async () => {
  if (!dirty.value) return true
  try {
    await ElMessageBox.confirm('有修改还没有保存，确定离开吗？', '离开页面', { type: 'warning', confirmButtonText: '离开', cancelButtonText: '留下' })
    return true
  } catch {
    return false
  }
})

function beforeUnload(e: BeforeUnloadEvent) {
  if (dirty.value) e.preventDefault()
}

onMounted(() => {
  window.addEventListener('beforeunload', beforeUnload)
  loadCategories()
  loadPlatforms()
  searchTags('')
  load()
})
onBeforeUnmount(() => window.removeEventListener('beforeunload', beforeUnload))
</script>

<template>
  <div v-loading="loading" class="edit">
    <div class="main">
      <el-input v-model="form.title" class="title" maxlength="255" :placeholder="form.type === 'resource' ? '资源标题，如：Everything 便携版，按文件名秒搜整个硬盘' : '文章标题'" />
      <RichEditor v-model="form.contentHtml" />

      <el-card shadow="never" class="panel">
        <template #header>摘要</template>
        <el-input v-model="form.summary" type="textarea" :rows="3" maxlength="500" show-word-limit placeholder="显示在列表和搜索结果里。留空时自动截取正文开头 120 字。" />
      </el-card>

      <el-card v-if="form.type === 'resource'" shadow="never" class="panel">
        <template #header>资源信息</template>
        <el-form label-width="80px">
          <el-row :gutter="16">
            <el-col :span="8">
              <el-form-item label="平台">
                <el-select v-model="form.resource.platform" filterable allow-create default-first-option clearable placeholder="选择或输入" style="width: 100%">
                  <el-option v-for="p in platforms" :key="p" :label="p" :value="p" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="版本号">
                <el-input v-model="form.resource.version" maxlength="64" placeholder="如：1.4.1.1026" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="文件大小">
                <el-input v-model="form.resource.fileSize" maxlength="32" placeholder="如：1.7 MB" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-form-item label="软件图标">
            <ImagePicker v-model="form.resource.icon" shape="square" hint="方形图标，没有封面图时显示在卡片上，底色按图标颜色自动搭配。" />
          </el-form-item>
          <el-form-item label="自定义属性">
            <div class="rows">
              <div v-for="(a, i) in form.resource.attrs" :key="i" class="row">
                <el-input v-model="a.name" placeholder="属性名，如：兼容系统" maxlength="32" style="width: 180px" />
                <el-input v-model="a.value" placeholder="属性值，如：Windows 7+" maxlength="255" />
                <el-button link type="danger" @click="form.resource.attrs.splice(i, 1)">删除</el-button>
              </div>
              <el-button size="small" :disabled="form.resource.attrs.length >= 20" @click="addAttr">添加属性</el-button>
            </div>
          </el-form-item>
          <el-form-item label="更新日志">
            <div class="rows">
              <div v-for="(l, i) in form.resource.changelog" :key="i" class="row log">
                <el-input v-model="l.version" placeholder="版本号" maxlength="64" style="width: 140px" />
                <el-date-picker v-model="l.date" type="date" value-format="YYYY-MM-DD" placeholder="日期" style="width: 150px" />
                <el-input v-model="l.note" type="textarea" :autosize="{ minRows: 1, maxRows: 4 }" placeholder="更新内容" maxlength="2000" />
                <el-button link type="danger" @click="form.resource.changelog.splice(i, 1)">删除</el-button>
              </div>
              <el-button size="small" :disabled="form.resource.changelog.length >= 50" @click="addLog">添加一条（新的在最上面）</el-button>
            </div>
          </el-form-item>
        </el-form>
      </el-card>

      <el-card shadow="never" class="panel">
        <template #header>SEO</template>
        <el-form label-width="80px">
          <el-form-item label="SEO 标题">
            <el-input v-model="form.seoTitle" maxlength="255" placeholder="留空时使用标题" />
          </el-form-item>
          <el-form-item label="关键词">
            <el-input v-model="form.seoKeywords" maxlength="255" placeholder="用英文逗号分隔" />
          </el-form-item>
          <el-form-item label="描述">
            <el-input v-model="form.seoDescription" type="textarea" :rows="2" maxlength="500" placeholder="留空时使用摘要" />
          </el-form-item>
        </el-form>
      </el-card>
    </div>

    <aside class="side">
      <el-card shadow="never" class="panel">
        <template #header>
          <div class="pub-h">
            <span>发布</span>
            <el-tag v-if="post" :type="post.status === 'published' ? (scheduled ? 'warning' : 'success') : 'info'" size="small">
              {{ post.status === 'published' ? (scheduled ? '定时发布' : '已发布') : post.status === 'trash' ? '回收站' : '草稿' }}
            </el-tag>
            <el-tag v-else size="small" type="info">未保存</el-tag>
          </div>
        </template>
        <el-form label-position="top">
          <el-form-item label="类型">
            <el-radio-group v-model="form.type">
              <el-radio-button value="resource">{{ typeLabel.resource }}</el-radio-button>
              <el-radio-button value="article">{{ typeLabel.article }}</el-radio-button>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="发布时间">
            <el-date-picker
              v-model="form.publishedAt"
              type="datetime"
              value-format="YYYY-MM-DDTHH:mm:ssZ"
              placeholder="留空则发布时取当前时间"
              style="width: 100%"
            />
            <p v-if="scheduled" class="note">发布后到这个时间才会在前台显示。</p>
          </el-form-item>
          <el-form-item>
            <el-checkbox v-model="form.pinned">置顶</el-checkbox>
          </el-form-item>
        </el-form>
        <div class="acts">
          <el-button :loading="saving" @click="save('draft')">{{ published ? '转为草稿' : '保存草稿' }}</el-button>
          <el-button type="primary" :loading="saving" @click="save('published')">{{ published ? '更新' : '发布' }}</el-button>
        </div>
        <p v-if="post" class="note">最后修改 {{ formatTime(post.updatedAt) }}<template v-if="dirty">，有未保存的修改</template></p>
      </el-card>

      <el-card shadow="never" class="panel">
        <template #header>分类</template>
        <el-tree-select
          v-model="form.categoryId"
          :data="tree"
          :props="{ label: 'name', children: 'children' }"
          node-key="id"
          check-strictly
          default-expand-all
          placeholder="选择分类"
          style="width: 100%"
        />
        <p v-if="!categories.length" class="note">还没有分类，<RouterLink :to="{ name: 'categories' }">先去新建</RouterLink>。</p>
      </el-card>

      <el-card shadow="never" class="panel">
        <template #header>标签</template>
        <el-select
          v-model="form.tags"
          multiple
          filterable
          remote
          allow-create
          default-first-option
          :remote-method="searchTags"
          :multiple-limit="10"
          placeholder="输入后回车，最多 10 个"
          style="width: 100%"
        >
          <el-option v-for="t in tagOptions" :key="t" :label="t" :value="t" />
        </el-select>
      </el-card>

      <el-card shadow="never" class="panel">
        <template #header>封面图</template>
        <ImagePicker v-model="form.cover" hint="建议 16:10，如 800 × 500。" />
      </el-card>
    </aside>
  </div>
</template>

<style scoped>
.edit { display: grid; grid-template-columns: minmax(0, 1fr) 300px; gap: 16px; align-items: start; }
.main, .side { display: flex; flex-direction: column; gap: 16px; min-width: 0; }
.side { position: sticky; top: 0; }
.title :deep(.el-input__wrapper) { padding: 6px 14px; }
.title :deep(.el-input__inner) { height: 40px; font-size: 20px; font-weight: 600; color: #262A31; }
.panel :deep(.el-card__header) { padding: 12px 16px; font-size: 14px; font-weight: 600; color: #262A31; }
.panel :deep(.el-card__body) { padding: 16px; }
.pub-h { display: flex; align-items: center; justify-content: space-between; }
.acts { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.acts .el-button { margin: 0; }
.note { margin: 6px 0 0; font-size: 12px; line-height: 1.6; color: #8A9099; }
.rows { display: flex; flex-direction: column; gap: 8px; width: 100%; align-items: flex-start; }
.row { display: flex; gap: 8px; width: 100%; align-items: flex-start; }
@media (max-width: 1100px) {
  .edit { grid-template-columns: minmax(0, 1fr); }
  .side { position: static; }
}
</style>
