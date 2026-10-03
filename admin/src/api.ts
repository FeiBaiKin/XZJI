import { createAdminClient } from '@xzji/api-client/admin'

// 后台接口与页面同源（开发时经 Vite 代理），登录 Cookie 只作用于后台域名
export const api = createAdminClient({ baseUrl: '/admin-api/v1' })
