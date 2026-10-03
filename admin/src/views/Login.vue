<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { errorMessage } from '@xzji/api-client/admin'
import { api } from '../api'
import { ensureMe, me } from '../auth'

const route = useRoute()
const router = useRouter()
const form = reactive({ email: '', password: '' })
const error = ref('')
const busy = ref(false)

function target() {
  const r = route.query.redirect
  return typeof r === 'string' && r.startsWith('/') && !r.startsWith('//') ? r : '/'
}

onMounted(async () => {
  if (await ensureMe()) await router.replace(target())
})

async function submit() {
  error.value = ''
  busy.value = true
  try {
    const { data, error: err } = await api.POST('/auth/login', { body: { ...form } })
    if (!data) {
      error.value = errorMessage(err)
      return
    }
    me.value = data
    await router.replace(target())
  } catch {
    error.value = errorMessage(null)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="page">
    <el-card class="box" shadow="never">
      <h1>XZJI 后台</h1>
      <p class="sub">仅限管理员登录</p>
      <el-form label-position="top" @submit.prevent="submit">
        <el-form-item label="邮箱">
          <el-input v-model="form.email" autocomplete="email" size="large" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" show-password autocomplete="current-password" size="large" />
        </el-form-item>
        <el-alert v-if="error" class="err" type="error" :title="error" show-icon :closable="false" />
        <el-button type="primary" native-type="submit" size="large" :loading="busy" class="submit">登录</el-button>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped>
.page { min-height: 100%; display: grid; place-items: center; padding: 20px; box-sizing: border-box; }
.box { width: 100%; max-width: 380px; border-radius: 10px; }
h1 { margin: 0; font-size: 20px; color: #262A31; }
.sub { margin: 4px 0 20px; font-size: 13px; color: #8A9099; }
.err { margin-bottom: 16px; }
.submit { width: 100%; }
</style>
