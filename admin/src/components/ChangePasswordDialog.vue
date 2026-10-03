<script setup lang="ts">
import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { errorMessage } from '@xzji/api-client/admin'
import { api } from '../api'

const open = defineModel<boolean>({ required: true })
const form = reactive({ oldPassword: '', newPassword: '', confirm: '' })
const error = ref('')
const busy = ref(false)

function reset() {
  form.oldPassword = form.newPassword = form.confirm = ''
  error.value = ''
}

async function submit() {
  error.value = ''
  if (form.newPassword !== form.confirm) {
    error.value = '两次输入的新密码不一致'
    return
  }
  busy.value = true
  try {
    const { error: err, response } = await api.POST('/me/password', {
      body: { oldPassword: form.oldPassword, newPassword: form.newPassword },
    })
    if (response.status !== 204) {
      error.value = errorMessage(err)
      return
    }
    ElMessage.success('密码已修改，其他设备上的登录已退出')
    open.value = false
  } catch {
    error.value = errorMessage(null)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <el-dialog v-model="open" title="修改密码" width="420px" @closed="reset">
    <el-form label-position="top" @submit.prevent="submit">
      <el-form-item label="原密码">
        <el-input v-model="form.oldPassword" type="password" show-password autocomplete="current-password" />
      </el-form-item>
      <el-form-item label="新密码">
        <el-input v-model="form.newPassword" type="password" show-password autocomplete="new-password" placeholder="至少 8 位，同时包含字母和数字" />
      </el-form-item>
      <el-form-item label="确认新密码">
        <el-input v-model="form.confirm" type="password" show-password autocomplete="new-password" />
      </el-form-item>
      <el-alert v-if="error" type="error" :title="error" show-icon :closable="false" />
    </el-form>
    <template #footer>
      <el-button @click="open = false">取消</el-button>
      <el-button type="primary" :loading="busy" @click="submit">修改密码</el-button>
    </template>
  </el-dialog>
</template>
