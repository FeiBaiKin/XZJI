// 记录推广链接 ?ref=用户ID，注册时作为邀请人提交，有效期 30 天。
// 只在浏览器端写 Cookie，避免服务端渲染的页面因为 Set-Cookie 而无法缓存。
export default defineNuxtRouteMiddleware((to) => {
  if (import.meta.server) return
  const ref = Number(to.query.ref)
  if (Number.isSafeInteger(ref) && ref > 0) {
    useCookie('xzji_ref', { maxAge: 30 * 24 * 3600, sameSite: 'lax', path: '/' }).value = String(ref)
  }
})
