// 后台接口客户端，类型由 api/admin.yaml 生成（task gen:api）
import createClient, { type ClientOptions } from 'openapi-fetch'
import { csrfMiddleware } from './csrf'
import type { components, paths } from './gen/admin'

export { errorCode, errorMessage } from './csrf'

export type AdminPaths = paths
export type AdminSchemas = components['schemas']

export function createAdminClient(options: ClientOptions) {
  const client = createClient<paths>(options)
  client.use(csrfMiddleware(async () => {
    const { data } = await client.GET('/auth/csrf')
    if (!data) throw new Error('获取 CSRF 令牌失败')
    return data.token
  }))
  return client
}

export type AdminClient = ReturnType<typeof createAdminClient>
