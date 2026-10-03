import { createPublicClient, type PublicClient } from '@xzji/api-client/public'

let browserClient: PublicClient | null = null

// 服务端渲染时走内网地址且不带用户身份，渲染结果可以给所有访客共用缓存；
// 浏览器端直连接口并携带 Cookie，用于获取个人数据。浏览器端共用一个客户端，CSRF 令牌只取一次。
export function useApi(): PublicClient {
  const config = useRuntimeConfig()
  if (import.meta.server) {
    return createPublicClient({ baseUrl: config.apiInternalBase })
  }
  browserClient ??= createPublicClient({ baseUrl: config.public.apiBase, credentials: 'include' })
  return browserClient
}
