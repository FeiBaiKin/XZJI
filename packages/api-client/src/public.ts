// 前台接口客户端，类型由 api/public.yaml 生成（task gen:api）
import createClient, { type ClientOptions } from 'openapi-fetch'
import { csrfMiddleware } from './csrf'
import type { components, paths } from './gen/public'

export { errorCode, errorMessage } from './csrf'

export type PublicPaths = paths
export type PublicSchemas = components['schemas']
export type ApiError = components['schemas']['Error']

export function createPublicClient(options: ClientOptions) {
  const client = createClient<paths>(options)
  client.use(csrfMiddleware(async () => {
    const { data } = await client.GET('/auth/csrf')
    if (!data) throw new Error('获取 CSRF 令牌失败')
    return data.token
  }))
  return client
}

export type PublicClient = ReturnType<typeof createPublicClient>
