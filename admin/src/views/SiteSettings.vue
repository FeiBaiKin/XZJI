<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { errorMessage, type AdminSchemas } from '@xzji/api-client/admin'
import { api } from '../api'
import RichEditor from '../components/RichEditor.vue'

const loading = ref(false)
const saving = ref(false)
const form = reactive<AdminSchemas['SiteSettingsInput']>({
  footerAbout: '', icp: '', policeIcp: '', termsHtml: '', privacyHtml: '',
})

// 隐私政策该写到的要点，只做提示，正文由站长自己写
const privacyPoints = [
  '收集哪些信息：邮箱、IP、订单与支付记录、Cookie',
  '这些信息用在什么地方',
  '会提供给哪些第三方：支付宝、微信支付',
  '保存多久，到期后怎么处理',
  '用户可以查询、更正自己的信息，以及注销账号的方式',
  '联系方式，方便用户提出请求',
]

function fill(data: AdminSchemas['SiteSettings']) {
  form.footerAbout = data.footerAbout
  form.icp = data.icp
  form.policeIcp = data.policeIcp
  form.termsHtml = data.termsHtml
  form.privacyHtml = data.privacyHtml
}

async function load() {
  loading.value = true
  const { data, error } = await api.GET('/settings/site')
  loading.value = false
  if (!data) return ElMessage.error(errorMessage(error))
  fill(data)
}

async function save() {
  saving.value = true
  const { data, error } = await api.PUT('/settings/site', { body: form })
  saving.value = false
  if (!data) return ElMessage.error(errorMessage(error))
  fill(data)
  ElMessage.success('已保存')
}

onMounted(load)
</script>

<template>
  <div v-loading="loading" class="site">
    <el-card shadow="never">
      <template #header>
        <div class="head"><span>页脚信息</span><small>显示在前台每个页面的页脚。</small></div>
      </template>
      <el-form label-width="96px">
        <el-form-item label="页脚简介">
          <el-input v-model="form.footerAbout" type="textarea" :rows="2" maxlength="200" show-word-limit placeholder="一句话说明本站提供什么内容，如：整理安卓、电脑软件和建站资源。" />
        </el-form-item>
        <el-form-item label="ICP 备案号">
          <el-input v-model="form.icp" maxlength="60" placeholder="如：京ICP备00000000号-1" />
        </el-form-item>
        <el-form-item label="公安备案号">
          <div class="block">
            <el-input v-model="form.policeIcp" maxlength="60" placeholder="没有就留空，如：京公网安备11010502000000号" />
            <small>填写后会在页脚显示，并链接到公安备案查询页（自动取备案号里的数字）。</small>
          </div>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never">
      <template #header>
        <div class="head"><span>用户协议</span><small>前台 /terms 页面显示，注册时也会链接到这里。</small></div>
      </template>
      <RichEditor v-model="form.termsHtml" :allow-hidden="false" />
      <small class="tip">正文与内容正文使用同一套过滤规则，脚本、样式和事件属性都会被去掉。留空时前台显示“站长尚未填写”。</small>
    </el-card>

    <el-card shadow="never">
      <template #header>
        <div class="head"><span>隐私政策</span><small>前台 /privacy 页面显示。正文请自己写，下面只是该写到的要点。</small></div>
      </template>
      <el-alert type="info" :closable="false" class="points">
        <template #title>建议包含以下内容</template>
        <ul>
          <li v-for="p in privacyPoints" :key="p">{{ p }}</li>
        </ul>
      </el-alert>
      <RichEditor v-model="form.privacyHtml" :allow-hidden="false" />
    </el-card>

    <div class="save">
      <el-button type="primary" :loading="saving" @click="save">保存站点设置</el-button>
    </div>
  </div>
</template>

<style scoped>
.site { display: flex; flex-direction: column; gap: 16px; max-width: 900px; }
.head { display: flex; align-items: baseline; gap: 12px; font-weight: 600; color: #262A31; }
.head small { font-size: 12px; font-weight: 400; color: #8A9099; }
.block { display: flex; flex-direction: column; gap: 4px; width: 100%; }
small { font-size: 12px; color: #8A9099; }
.tip { display: block; margin-top: 8px; }
.points { margin-bottom: 12px; }
.points ul { margin: 4px 0 0; padding-left: 18px; }
.points li { line-height: 1.8; }
.save { position: sticky; bottom: 0; padding: 12px 0; background: #F4F5F7; }
</style>
