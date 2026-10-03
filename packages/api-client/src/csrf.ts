import type { Middleware } from 'openapi-fetch'

const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS'])

// 写请求自动带上 CSRF 令牌：首次写请求前取一次令牌并缓存；
// 服务端返回 CSRF_INVALID（令牌过期）时清掉缓存，下次请求重新获取。
export function csrfMiddleware(fetchToken: () => Promise<string>): Middleware {
  let token: Promise<string> | null = null
  return {
    async onRequest({ request }) {
      if (SAFE_METHODS.has(request.method)) return undefined
      token ??= fetchToken().catch((err: unknown) => {
        token = null
        throw err
      })
      request.headers.set('X-CSRF-Token', await token)
      return request
    },
    async onResponse({ response }) {
      if (response.status !== 403) return undefined
      const body = await response.clone().json().catch(() => null) as { error?: { code?: string } } | null
      if (body?.error?.code === 'CSRF_INVALID') token = null
      return undefined
    },
  }
}

export type ErrorBody = { error: { code: string, message: string } }

// 取出可以直接展示给用户的错误提示
export function errorMessage(error: unknown, fallback = '网络异常，请稍后重试'): string {
  const e = error as Partial<ErrorBody> | null | undefined
  return e?.error?.message ?? fallback
}

export function errorCode(error: unknown): string | undefined {
  return (error as Partial<ErrorBody> | null | undefined)?.error?.code
}
