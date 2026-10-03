// 需要登录的页面：未登录时跳到登录页，登录后回到原页面
export default defineNuxtRouteMiddleware(async (to) => {
  if (import.meta.server) return
  const user = await useAuth().ensure()
  if (!user) {
    return navigateTo({ path: '/login', query: { redirect: to.fullPath } })
  }
})
