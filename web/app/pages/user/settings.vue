<script setup lang="ts">
import { errorMessage } from '@xzji/api-client/public'

definePageMeta({ middleware: 'auth' })

const api = useApi()
const auth = useAuth()
const user = computed(() => auth.user.value!)

// ---------- 昵称 ----------
const nickname = ref(user.value.nickname)
const nickBusy = ref(false)
const nickMsg = ref<{ ok: boolean, text: string } | null>(null)

async function saveNickname() {
  nickMsg.value = null
  nickBusy.value = true
  try {
    const { data, error } = await api.PATCH('/me', { body: { nickname: nickname.value } })
    if (!data) {
      nickMsg.value = { ok: false, text: errorMessage(error) }
      return
    }
    auth.user.value = data
    nickMsg.value = { ok: true, text: '昵称已保存' }
  } catch {
    nickMsg.value = { ok: false, text: errorMessage(null) }
  } finally {
    nickBusy.value = false
  }
}

// ---------- 密码 ----------
const pw = reactive({ old: '', next: '', confirm: '' })
const pwBusy = ref(false)
const pwMsg = ref<{ ok: boolean, text: string } | null>(null)

async function changePassword() {
  pwMsg.value = null
  if (pw.next !== pw.confirm) {
    pwMsg.value = { ok: false, text: '两次输入的新密码不一致' }
    return
  }
  pwBusy.value = true
  try {
    const { error, response } = await api.POST('/me/password', { body: { oldPassword: pw.old, newPassword: pw.next } })
    if (response.status !== 204) {
      pwMsg.value = { ok: false, text: errorMessage(error) }
      return
    }
    pw.old = pw.next = pw.confirm = ''
    pwMsg.value = { ok: true, text: '密码已修改，其他设备上的登录已退出' }
  } catch {
    pwMsg.value = { ok: false, text: errorMessage(null) }
  } finally {
    pwBusy.value = false
  }
}
</script>

<template>
  <UserShell title="账号设置">
    <div class="flex flex-col gap-5">
      <section class="card px-6 py-5 max-md:px-4">
        <h2 class="relative pl-3 text-[17px] font-bold text-title before:absolute before:inset-y-0.5 before:left-0 before:w-1 before:rounded before:bg-theme">个人资料</h2>
        <form class="mt-4 flex max-w-[420px] flex-col gap-3" @submit.prevent="saveNickname">
          <label>
            <span class="field-label">昵称</span>
            <input v-model="nickname" required maxlength="20" class="field-input" placeholder="2 到 20 个字符">
          </label>
          <p v-if="nickMsg" class="text-[13px]" :class="nickMsg.ok ? 'text-success' : 'text-danger'">{{ nickMsg.text }}</p>
          <button type="submit" class="btn w-fit bg-jb-blue" :disabled="nickBusy || nickname === user.nickname">保存</button>
        </form>
      </section>

      <section class="card px-6 py-5 max-md:px-4">
        <h2 class="relative pl-3 text-[17px] font-bold text-title before:absolute before:inset-y-0.5 before:left-0 before:w-1 before:rounded before:bg-theme">修改密码</h2>
        <form class="mt-4 flex max-w-[420px] flex-col gap-3" @submit.prevent="changePassword">
          <label>
            <span class="field-label">原密码</span>
            <input v-model="pw.old" type="password" autocomplete="current-password" required class="field-input">
          </label>
          <label>
            <span class="field-label">新密码</span>
            <input v-model="pw.next" type="password" autocomplete="new-password" required class="field-input" placeholder="至少 8 位，同时包含字母和数字">
          </label>
          <label>
            <span class="field-label">确认新密码</span>
            <input v-model="pw.confirm" type="password" autocomplete="new-password" required class="field-input">
          </label>
          <p v-if="pwMsg" class="text-[13px]" :class="pwMsg.ok ? 'text-success' : 'text-danger'">{{ pwMsg.text }}</p>
          <button type="submit" class="btn w-fit bg-jb-blue" :disabled="pwBusy">修改密码</button>
        </form>
      </section>
    </div>
  </UserShell>
</template>
