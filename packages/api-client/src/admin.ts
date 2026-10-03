// 后台接口客户端，类型由 api/admin.yaml 生成（task gen:api）
import createClient, { type ClientOptions } from 'openapi-fetch'
import type { components, paths } from './gen/admin'

export type AdminPaths = paths
export type AdminSchemas = components['schemas']

export function createAdminClient(options: ClientOptions) {
  return createClient<paths>(options)
}

export type AdminClient = ReturnType<typeof createAdminClient>
