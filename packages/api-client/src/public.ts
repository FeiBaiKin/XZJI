// 前台接口客户端，类型由 api/public.yaml 生成（task gen:api）
import createClient, { type ClientOptions } from 'openapi-fetch'
import type { components, paths } from './gen/public'

export type PublicPaths = paths
export type PublicSchemas = components['schemas']
export type ApiError = components['schemas']['Error']

export function createPublicClient(options: ClientOptions) {
  return createClient<paths>(options)
}

export type PublicClient = ReturnType<typeof createPublicClient>
