<script setup lang="ts">
import { errorMessage } from '@xzji/api-client/public'

definePageMeta({ layout: 'auth' })
useSeoMeta({ title: '登录 XZJI', robots: 'noindex' })

const route = useRoute()
const api = useApi()
const auth = useAuth()
const redirect = computed(() => safeRedirect(route.query.redirect))

const tab = ref<'login' | 'register'>(route.query.tab === 'register' ? 'register' : 'login')
watch(() => route.query.tab, (v) => { tab.value = v === 'register' ? 'register' : 'login' })

// 已登录时直接跳走
onMounted(async () => {
  if (await auth.ensure()) await navigateTo(redirect.value, { replace: true })
})

// ---------- 登录 ----------
const login = reactive({ email: typeof route.query.email === 'string' ? route.query.email : '', password: '', remember: true })
const loginError = ref('')
const loginBusy = ref(false)

async function submitLogin() {
  loginError.value = ''
  loginBusy.value = true
  try {
    const { data, error } = await api.POST('/auth/login', { body: { ...login } })
    if (!data) {
      loginError.value = errorMessage(error)
      return
    }
    auth.user.value = data.user
    await navigateTo(redirect.value, { replace: true })
  } catch {
    loginError.value = errorMessage(null)
  } finally {
    loginBusy.value = false
  }
}

// ---------- 注册 ----------
const reg = reactive({ email: '', code: '', password: '', agree: false })
const regError = ref('')
const regNotice = ref('')
const regBusy = ref(false)
const sending = ref(false)
const countdown = useCountdown(60)

const refCookie = useCookie('xzji_ref')
const inviterId = computed(() => {
  const v = Number(refCookie.value)
  return Number.isSafeInteger(v) && v > 0 ? v : undefined
})

async function sendCode() {
  regError.value = ''
  regNotice.value = ''
  if (!reg.email.trim()) {
    regError.value = '请先填写邮箱'
    return
  }
  sending.value = true
  try {
    const { error, response } = await api.POST('/auth/email-codes', { body: { email: reg.email, purpose: 'register' } })
    if (response.status !== 204) {
      regError.value = errorMessage(error)
      return
    }
    regNotice.value = `验证码已发送到 ${reg.email.trim()}，5 分钟内有效`
    countdown.start()
  } catch {
    regError.value = errorMessage(null)
  } finally {
    sending.value = false
  }
}

async function submitRegister() {
  regError.value = ''
  if (!reg.agree) {
    regError.value = '请先阅读并同意用户协议和隐私政策'
    return
  }
  regBusy.value = true
  try {
    const { data, error } = await api.POST('/auth/register', {
      body: { email: reg.email, code: reg.code.trim(), password: reg.password, inviterId: inviterId.value },
    })
    if (!data) {
      regError.value = errorMessage(error)
      return
    }
    auth.user.value = data.user
    refCookie.value = null
    await navigateTo(redirect.value, { replace: true })
  } catch {
    regError.value = errorMessage(null)
  } finally {
    regBusy.value = false
  }
}

function switchTab(t: 'login' | 'register') {
  navigateTo({ query: { ...route.query, tab: t === 'register' ? 'register' : undefined } }, { replace: true })
}
</script>

<template>
  <section class="card grid w-full max-w-[880px] grid-cols-2 overflow-hidden max-md:grid-cols-1">
    <AuthSide :title="tab === 'login' ? '登录 XZJI' : '注册 XZJI'" />

    <div class="px-10 py-9 max-md:px-5 max-md:py-6">
      <nav class="mb-5 flex gap-7 border-b border-line" role="tablist">
        <button
          v-for="t in (['login', 'register'] as const)"
          :key="t"
          role="tab"
          :aria-selected="tab === t"
          class="relative pb-2.5 text-[17px]"
          :class="tab === t ? 'font-bold text-title after:absolute after:inset-x-0 after:-bottom-px after:h-[3px] after:rounded after:bg-theme' : 'text-muted'"
          @click="switchTab(t)"
        >
          {{ t === 'login' ? '登录' : '注册' }}
        </button>
      </nav>

      <form v-if="tab === 'login'" class="flex flex-col gap-4" @submit.prevent="submitLogin">
        <label>
          <span class="field-label">邮箱</span>
          <input v-model="login.email" type="email" autocomplete="email" required class="field-input" placeholder="you@example.com">
        </label>
        <label class="relative">
          <span class="field-label">密码</span>
          <NuxtLink :to="{ path: '/forgot', query: login.email ? { email: login.email } : {} }" class="absolute top-0 right-0 text-[12.5px] text-theme">忘记密码？</NuxtLink>
          <input v-model="login.password" type="password" autocomplete="current-password" required class="field-input" placeholder="输入密码">
        </label>
        <label class="flex items-center gap-2 text-[13px]">
          <input v-model="login.remember" type="checkbox" class="size-[15px] accent-theme">30 天内自动登录
        </label>
        <p v-if="loginError" role="alert" class="rounded-md bg-danger-soft px-3 py-2 text-[13px] text-danger">{{ loginError }}</p>
        <button type="submit" class="btn h-11 w-full bg-jb-blue text-[15px]" :disabled="loginBusy">{{ loginBusy ? '登录中…' : '登录' }}</button>
        <p class="text-center text-[13px] text-muted">还没有账号？<button type="button" class="text-theme" @click="switchTab('register')">免费注册</button></p>
      </form>

      <form v-else class="flex flex-col gap-4" @submit.prevent="submitRegister">
        <p v-if="inviterId" class="flex items-center gap-2 rounded-md bg-[#FFF4E5] px-3 py-2.5 text-[13px] text-[#B45309]">
          <Icon name="gift" />你正在通过好友的邀请链接注册
        </p>
        <label>
          <span class="field-label">邮箱</span>
          <input v-model="reg.email" type="email" autocomplete="email" required class="field-input" placeholder="you@example.com">
        </label>
        <div>
          <label for="reg-code" class="field-label">验证码</label>
          <div class="flex gap-2.5">
            <input id="reg-code" v-model="reg.code" inputmode="numeric" maxlength="6" required class="field-input" placeholder="6 位数字">
            <button type="button" class="btn btn-soft h-[42px] shrink-0" :disabled="sending || countdown.left.value > 0" @click="sendCode">
              {{ countdown.left.value > 0 ? `${countdown.left.value} 秒后重发` : sending ? '发送中…' : '发送验证码' }}
            </button>
          </div>
          <p v-if="regNotice" class="mt-1 text-xs text-success">{{ regNotice }}</p>
        </div>
        <label>
          <span class="field-label">设置密码</span>
          <input v-model="reg.password" type="password" autocomplete="new-password" required class="field-input" placeholder="至少 8 位">
          <span class="mt-1 block text-xs text-muted">需要同时包含字母和数字</span>
        </label>
        <label class="flex items-center gap-2 text-[13px]">
          <input v-model="reg.agree" type="checkbox" class="size-[15px] accent-theme">我已阅读并同意<a href="#" class="text-theme">《用户协议》</a>和<a href="#" class="text-theme">《隐私政策》</a>
        </label>
        <p v-if="regError" role="alert" class="rounded-md bg-danger-soft px-3 py-2 text-[13px] text-danger">{{ regError }}</p>
        <button type="submit" class="btn h-11 w-full bg-jb-blue text-[15px]" :disabled="regBusy">{{ regBusy ? '注册中…' : '注册' }}</button>
        <p class="text-center text-[13px] text-muted">已经有账号？<button type="button" class="text-theme" @click="switchTab('login')">直接登录</button></p>
      </form>
    </div>
  </section>
</template>
