import { createAdminClient, errorMessage, type AdminSchemas } from '@xzji/api-client/admin'

// 后台接口与页面同源（开发时经 Vite 代理），登录 Cookie 只作用于后台域名
export const api = createAdminClient({ baseUrl: '/admin-api/v1' })

// 会话过期（如 12 小时到期、账号被取消管理员）时回到登录页
api.use({
  onResponse({ response, request }) {
    if (response.status === 401 && !request.url.endsWith('/me') && !location.pathname.endsWith('/login')) {
      const back = location.pathname.replace(/^\/admin/, '') + location.search
      location.assign(`${import.meta.env.BASE_URL}login?redirect=${encodeURIComponent(back)}`)
    }
    return undefined
  },
})

export function formatTime(value?: string | null) {
  if (!value) return '-'
  const d = new Date(value)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export type Media = AdminSchemas['Media']

// 上传一张图片，失败时抛出可直接展示的错误提示
export async function uploadImage(file: File): Promise<Media> {
  const form = new FormData()
  form.append('file', file)
  const { data, error } = await api.POST('/media', { body: { file: '' }, bodySerializer: () => form })
  if (!data) throw new Error(errorMessage(error, '上传失败，请稍后重试'))
  return data
}
