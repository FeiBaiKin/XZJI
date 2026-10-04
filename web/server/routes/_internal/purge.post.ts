// 后端内容变更后调用，清空整站页面缓存。
// 签名 = hex(HMAC-SHA256(密钥, 时间戳))，时间戳超过 5 分钟视为重放，直接拒绝。
// 用 Web Crypto 而不是 node:crypto，前台就不必引入 Node 类型依赖。
export default defineEventHandler(async (event) => {
  const secret = useRuntimeConfig().purgeSecret
  const timestamp = getHeader(event, 'x-purge-timestamp') ?? ''
  const signature = getHeader(event, 'x-purge-signature') ?? ''
  const seconds = Number(timestamp)
  const expired = Math.abs(Date.now() / 1000 - seconds) > 300
  const expected = secret ? await sign(secret, timestamp) : ''
  if (!secret || !Number.isSafeInteger(seconds) || expired || !equal(signature, expected)) {
    throw createError({ statusCode: 401, statusMessage: '签名校验失败' })
  }
  await clearCache()
  setResponseStatus(event, 204)
  return null
})

// 清空页面缓存。Nitro 打包后的 useStorage('cache').clear() 是空操作（前缀存储没把 clear 前缀化，
// 实测键还在），所以先调用它，再逐个删掉剩下的键，保证真的清干净。
async function clearCache(): Promise<void> {
  const cache = useStorage('cache')
  await cache.clear()
  for (const key of await cache.getKeys()) {
    await cache.removeItem(key)
  }
}

async function sign(secret: string, message: string): Promise<string> {
  const encoder = new TextEncoder()
  const key = await crypto.subtle.importKey('raw', encoder.encode(secret), { name: 'HMAC', hash: 'SHA-256' }, false, ['sign'])
  const mac = await crypto.subtle.sign('HMAC', key, encoder.encode(message))
  return [...new Uint8Array(mac)].map(b => b.toString(16).padStart(2, '0')).join('')
}

// 常量时间比较，避免通过响应时间猜签名。
function equal(a: string, b: string): boolean {
  let diff = a.length ^ b.length
  for (let i = 0; i < a.length; i++) {
    diff |= a.charCodeAt(i) ^ (b.charCodeAt(i) || 0)
  }
  return diff === 0
}
