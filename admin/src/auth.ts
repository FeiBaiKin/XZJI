import { ref } from 'vue'
import type { AdminSchemas } from '@xzji/api-client/admin'
import { api } from './api'

export const me = ref<AdminSchemas['AdminProfile'] | null>(null)
let loaded = false

// 首次进入页面时获取一次当前管理员，之后由登录、退出操作更新
export async function ensureMe() {
  if (!loaded) {
    const { data } = await api.GET('/auth/session')
    me.value = data?.admin ?? null
    loaded = true
  }
  return me.value
}

export async function logout() {
  await api.POST('/auth/logout')
  me.value = null
}
