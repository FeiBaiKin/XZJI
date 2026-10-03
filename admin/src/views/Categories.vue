<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { errorMessage, type AdminSchemas } from '@xzji/api-client/admin'
import { api } from '../api'
import { buildTree, categories, gradients, iconNames, loadCategories, mountIconSprite, type Category } from '../content'

type Form = Omit<AdminSchemas['CategoryInput'], 'parentId'> & { parentId: number | null }

const loading = ref(false)
const tree = computed(() => buildTree(categories.value))
const roots = computed(() => categories.value.filter(c => !c.parentId))

const dialog = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)
const empty = (): Form => ({ parentId: null, name: '', slug: '', icon: '', color: '', description: '', sort: 0, seoTitle: '', seoDescription: '' })
const form = reactive<Form>(empty())

async function reload() {
  loading.value = true
  await loadCategories(true)
  loading.value = false
}

function open(row?: Category, parentId: number | null = null) {
  editingId.value = row?.id ?? null
  Object.assign(form, empty(), row ? { ...row } : { parentId })
  dialog.value = true
}

async function save() {
  saving.value = true
  const body = { ...form, parentId: form.parentId || null }
  const { data, error } = editingId.value
    ? await api.PUT('/categories/{id}', { params: { path: { id: editingId.value } }, body })
    : await api.POST('/categories', { body })
  saving.value = false
  if (!data) {
    ElMessage.error(errorMessage(error))
    return
  }
  dialog.value = false
  ElMessage.success('已保存')
  await reload()
}

async function remove(row: Category) {
  try {
    await ElMessageBox.confirm(`确定删除分类“${row.name}”吗？`, '删除分类', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
  } catch {
    return
  }
  const { error, response } = await api.DELETE('/categories/{id}', { params: { path: { id: row.id } } })
  if (!response.ok) {
    ElMessage.error(errorMessage(error))
    return
  }
  ElMessage.success('已删除')
  await reload()
}

onMounted(() => {
  mountIconSprite()
  reload()
})
</script>

<template>
  <el-card shadow="never">
    <div class="toolbar">
      <p class="tip">文章和资源共用一套分类，最多两级。顶级分类会出现在前台导航和首页入口里，排序值越小越靠前。</p>
      <el-button type="primary" @click="open()">新建分类</el-button>
    </div>
    <el-table v-loading="loading" :data="tree" row-key="id" default-expand-all border empty-text="还没有分类，先新建一个">
      <el-table-column label="名称" min-width="220">
        <template #default="{ row }">
          <span class="name">
            <span class="ico" :style="{ background: gradients[row.color]?.css ?? gradients.blue!.css }">
              <svg v-if="row.icon"><use :href="`#i-${row.icon}`" /></svg>
            </span>
            {{ row.name }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="slug" label="Slug" min-width="140" />
      <el-table-column label="网址" min-width="160">
        <template #default="{ row }"><code>/category/{{ row.slug }}/</code></template>
      </el-table-column>
      <el-table-column prop="postCount" label="内容数" width="90" />
      <el-table-column prop="sort" label="排序" width="80" />
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="open(row)">编辑</el-button>
          <el-button v-if="!row.parentId" link type="primary" @click="open(undefined, row.id)">添加子分类</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <el-dialog v-model="dialog" :title="editingId ? '编辑分类' : '新建分类'" width="560px" destroy-on-close>
    <el-form label-width="88px" @submit.prevent="save">
      <el-form-item label="上级分类">
        <el-select v-model="form.parentId" placeholder="无（顶级分类）" clearable style="width: 100%">
          <el-option v-for="c in roots" :key="c.id" :label="c.name" :value="c.id" :disabled="c.id === editingId" />
        </el-select>
      </el-form-item>
      <el-form-item label="名称" required>
        <el-input v-model="form.name" maxlength="32" placeholder="如：电脑软件" />
      </el-form-item>
      <el-form-item label="Slug" required>
        <el-input v-model="form.slug" maxlength="64" placeholder="如：pc，只能用小写字母、数字和短横线" />
      </el-form-item>
      <el-form-item label="图标">
        <el-select v-model="form.icon" placeholder="不设置" clearable filterable style="width: 100%">
          <el-option v-for="name in iconNames" :key="name" :label="name" :value="name">
            <span class="opt"><svg><use :href="`#i-${name}`" /></svg>{{ name }}</span>
          </el-option>
        </el-select>
      </el-form-item>
      <el-form-item label="颜色">
        <div class="swatches">
          <button type="button" class="sw none" :class="{ on: !form.color }" title="默认" @click="form.color = ''">默认</button>
          <button
            v-for="(g, key) in gradients"
            :key="key"
            type="button"
            class="sw"
            :class="{ on: form.color === key }"
            :title="g.label"
            :style="{ background: g.css }"
            @click="form.color = key"
          />
        </div>
      </el-form-item>
      <el-form-item label="描述">
        <el-input v-model="form.description" type="textarea" :rows="2" maxlength="255" show-word-limit placeholder="显示在分类页顶部" />
      </el-form-item>
      <el-form-item label="排序">
        <el-input-number v-model="form.sort" :min="-9999" :max="9999" controls-position="right" />
      </el-form-item>
      <el-form-item label="SEO 标题">
        <el-input v-model="form.seoTitle" maxlength="255" placeholder="留空时使用分类名称" />
      </el-form-item>
      <el-form-item label="SEO 描述">
        <el-input v-model="form.seoDescription" type="textarea" :rows="2" maxlength="500" placeholder="留空时使用分类描述" />
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
.name { display: inline-flex; align-items: center; gap: 8px; color: #262A31; }
.ico { width: 24px; height: 24px; border-radius: 6px; display: inline-grid; place-items: center; color: #fff; }
.ico svg, .opt svg { width: 14px; height: 14px; fill: none; stroke: currentColor; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; }
.opt { display: inline-flex; align-items: center; gap: 8px; }
.swatches { display: flex; flex-wrap: wrap; gap: 8px; }
.sw { width: 32px; height: 24px; border: 2px solid transparent; border-radius: 5px; cursor: pointer; padding: 0; }
.sw.none { width: auto; padding: 0 8px; background: #F6F7F9; font-size: 12px; color: #8A9099; }
.sw.on { border-color: #262A31; box-shadow: 0 0 0 2px #fff inset; }
code { font-size: 12px; color: #8A9099; }
</style>
