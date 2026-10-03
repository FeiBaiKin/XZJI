<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { errorMessage, type AdminSchemas } from '@xzji/api-client/admin'
import { api, type Media } from '../api'
import ImagePicker from '../components/ImagePicker.vue'

type Banner = Omit<AdminSchemas['BannerSetting'], 'image'> & { image: Media | null }

const maxBanners = 5
const loading = ref(false)
const saving = ref(false)
const form = reactive<{ banners: Banner[], notice: string }>({ banners: [], notice: '' })

function fill(data: AdminSchemas['HomeSettings']) {
  form.banners = data.banners.map(b => ({ ...b, image: b.image ?? null }))
  form.notice = data.notice
}

async function load() {
  loading.value = true
  const { data, error } = await api.GET('/settings/home')
  loading.value = false
  if (!data) return ElMessage.error(errorMessage(error))
  fill(data)
}

function add() {
  form.banners.push({ tag: '', title: '', subtitle: '', buttonText: '查看详情', link: '/', image: null })
}

function move(i: number, d: -1 | 1) {
  const list = form.banners
  ;[list[i], list[i + d]] = [list[i + d]!, list[i]!]
}

async function save() {
  saving.value = true
  const { data, error } = await api.PUT('/settings/home', {
    body: {
      banners: form.banners.map(({ image, ...b }) => ({ ...b, imageId: image?.id ?? null })),
      notice: form.notice,
    },
  })
  saving.value = false
  if (!data) return ElMessage.error(errorMessage(error))
  fill(data)
  ElMessage.success('已保存')
}

onMounted(load)
</script>

<template>
  <div v-loading="loading" class="home">
    <el-card shadow="never">
      <template #header>
        <div class="head">
          <span>首页横幅</span>
          <small>最多 {{ maxBanners }} 张，按顺序轮播。没有设置时前台显示站点名称。</small>
        </div>
      </template>
      <div v-for="(b, i) in form.banners" :key="i" class="banner">
        <div class="no">{{ i + 1 }}</div>
        <el-form label-width="72px" class="fields">
          <el-form-item label="标题" required>
            <el-input v-model="b.title" type="textarea" :autosize="{ minRows: 1, maxRows: 3 }" maxlength="60" show-word-limit placeholder="可以换行，如：重装系统后，&#10;先装这 12 个软件" />
          </el-form-item>
          <el-form-item label="小标签">
            <el-input v-model="b.tag" maxlength="12" placeholder="标题上方的小字，如：装机必备合集" />
          </el-form-item>
          <el-form-item label="说明">
            <el-input v-model="b.subtitle" maxlength="120" placeholder="一句话说明" />
          </el-form-item>
          <el-form-item label="按钮" required>
            <div class="pair">
              <el-input v-model="b.buttonText" maxlength="12" placeholder="按钮文字" style="width: 140px" />
              <el-input v-model="b.link" maxlength="500" placeholder="链接，站内用 / 开头，如 /123.html" />
            </div>
          </el-form-item>
          <el-form-item label="背景图">
            <ImagePicker v-model="b.image" hint="建议 1600 × 600，文字会叠在图片左侧。不传时使用默认渐变。" />
          </el-form-item>
        </el-form>
        <div class="ops">
          <el-button link :disabled="i === 0" @click="move(i, -1)">上移</el-button>
          <el-button link :disabled="i === form.banners.length - 1" @click="move(i, 1)">下移</el-button>
          <el-button link type="danger" @click="form.banners.splice(i, 1)">删除</el-button>
        </div>
      </div>
      <el-button :disabled="form.banners.length >= maxBanners" @click="add">添加横幅</el-button>
    </el-card>

    <el-card shadow="never">
      <template #header>
        <div class="head"><span>本站公告</span><small>显示在首页侧栏，每行一段。</small></div>
      </template>
      <el-input v-model="form.notice" type="textarea" :rows="4" maxlength="1000" show-word-limit placeholder="如：遇到下载地址失效，在资源页点“链接失效”反馈，一般 24 小时内补上。" />
    </el-card>

    <div class="save">
      <el-button type="primary" :loading="saving" @click="save">保存首页设置</el-button>
    </div>
  </div>
</template>

<style scoped>
.home { display: flex; flex-direction: column; gap: 16px; max-width: 900px; }
.head { display: flex; align-items: baseline; gap: 12px; font-weight: 600; color: #262A31; }
.head small { font-size: 12px; font-weight: 400; color: #8A9099; }
.banner { display: flex; gap: 12px; padding: 16px 0; border-bottom: 1px dashed #EEF0F3; }
.banner:first-child { padding-top: 0; }
.no { flex: none; width: 24px; height: 24px; border-radius: 6px; background: #EBF1FF; color: #2F6BFF; font-size: 13px; font-weight: 700; display: grid; place-items: center; }
.fields { flex: 1; min-width: 0; }
.pair { display: flex; gap: 8px; width: 100%; }
.ops { flex: none; display: flex; flex-direction: column; align-items: flex-start; gap: 4px; }
.ops .el-button { margin: 0; }
.banner + .el-button { margin-top: 16px; }
.save { position: sticky; bottom: 0; padding: 12px 0; background: #F4F5F7; }
</style>
