<script setup lang="ts">
import { errorMessage } from '@xzji/api-client/public'

definePageMeta({ layout: 'auth' })
useSeoMeta({ title: '找回密码 - XZJI', robots: 'noindex' })

const route = useRoute()
const api = useApi()

const form = reactive({
  email: typeof route.query.email === 'string' ? route.query.email : '',
  code: '',
  password: '',
  confirm: '',
})
const error = ref('')
const notice = ref('')
const busy = ref(false)
const sending = ref(false)
const done = ref(false)
const countdown = useCountdown(60)

async function sendCode() {
  error.value = ''
  notice.value = ''
  if (!form.email.trim()) {
    error.value = '请先填写注册时使用的邮箱'
    return
  }
  sending.value = true
  try {
    const { error: err, response } = await api.POST('/auth/email-codes', { body: { email: form.email, purpose: 'reset_password' } })
    if (response.status !== 204) {
      error.value = errorMessage(err)
      return
    }
    // 不论邮箱是否注册都提示同样的内容，不暴露注册情况
    notice.value = `如果 ${form.email.trim()} 已注册，验证码会发到这个邮箱，5 分钟内有效`
    countdown.start()
  } catch {
    error.value = errorMessage(null)
  } finally {
    sending.value = false
  }
}

async function submit() {
  error.value = ''
  if (form.password !== form.confirm) {
    error.value = '两次输入的新密码不一致'
    return
  }
  busy.value = true
  try {
    const { error: err, response } = await api.POST('/auth/password-reset', {
      body: { email: form.email, code: form.code.trim(), password: form.password },
    })
    if (response.status !== 204) {
      error.value = errorMessage(err)
      return
    }
    done.value = true
  } catch {
    error.value = errorMessage(null)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="card w-full max-w-[440px] px-10 py-9 max-md:px-5 max-md:py-6">
    <h1 class="text-xl font-bold text-title">找回密码</h1>

    <div v-if="done" class="mt-5">
      <p class="rounded-md bg-[#E6F8F1] px-3 py-2.5 text-[13px] text-[#0E9F78]">密码已重设，之前登录的设备都已退出。请用新密码登录。</p>
      <NuxtLink :to="{ path: '/login', query: { email: form.email } }" class="btn mt-4 h-11 w-full bg-jb-blue text-[15px]">去登录</NuxtLink>
    </div>

    <form v-else class="mt-5 flex flex-col gap-4" @submit.prevent="submit">
      <label>
        <span class="field-label">邮箱</span>
        <input v-model="form.email" type="email" autocomplete="email" required class="field-input" placeholder="注册时使用的邮箱">
      </label>
      <div>
        <label for="reset-code" class="field-label">验证码</label>
        <div class="flex gap-2.5">
          <input id="reset-code" v-model="form.code" inputmode="numeric" maxlength="6" required class="field-input" placeholder="6 位数字">
          <button type="button" class="btn btn-soft h-[42px] shrink-0" :disabled="sending || countdown.left.value > 0" @click="sendCode">
            {{ countdown.left.value > 0 ? `${countdown.left.value} 秒后重发` : sending ? '发送中…' : '发送验证码' }}
          </button>
        </div>
        <p v-if="notice" class="mt-1 text-xs text-success">{{ notice }}</p>
      </div>
      <label>
        <span class="field-label">新密码</span>
        <input v-model="form.password" type="password" autocomplete="new-password" required class="field-input" placeholder="至少 8 位，同时包含字母和数字">
      </label>
      <label>
        <span class="field-label">确认新密码</span>
        <input v-model="form.confirm" type="password" autocomplete="new-password" required class="field-input" placeholder="再输入一次">
      </label>
      <p v-if="error" role="alert" class="rounded-md bg-danger-soft px-3 py-2 text-[13px] text-danger">{{ error }}</p>
      <button type="submit" class="btn h-11 w-full bg-jb-blue text-[15px]" :disabled="busy">{{ busy ? '提交中…' : '重设密码' }}</button>
      <p class="text-center text-[13px] text-muted">想起来了？<NuxtLink to="/login" class="text-theme">返回登录</NuxtLink></p>
    </form>
  </section>
</template>
