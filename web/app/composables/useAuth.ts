import type { PublicSchemas } from '@xzji/api-client/public'

type User = PublicSchemas['User']

// 登录状态只在浏览器端获取：服务端渲染的页面不带用户身份，才能给所有访客共用缓存。
export function useAuth() {
  const user = useState<User | null>('auth:user', () => null)
  const ready = useState('auth:ready', () => false)

  async function refresh() {
    const { data } = await useApi().GET('/auth/session')
    user.value = data?.user ?? null
    ready.value = true
  }

  async function ensure() {
    if (!ready.value) await refresh()
    return user.value
  }

  async function logout() {
    await useApi().POST('/auth/logout')
    user.value = null
  }

  return { user, ready, refresh, ensure, logout }
}

// 登录后的跳转地址只允许站内相对路径，防止被利用跳到外部网站
export function safeRedirect(value: unknown, fallback = '/user'): string {
  return typeof value === 'string' && value.startsWith('/') && !value.startsWith('//') ? value : fallback
}
