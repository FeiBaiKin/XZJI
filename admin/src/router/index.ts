import { createRouter, createWebHistory } from 'vue-router'
import { ensureMe } from '../auth'

export const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/login', name: 'login', component: () => import('../views/Login.vue'), meta: { title: '登录', public: true } },
    {
      path: '/',
      component: () => import('../layouts/AdminLayout.vue'),
      children: [
        { path: '', name: 'dashboard', component: () => import('../views/Dashboard.vue'), meta: { title: '仪表盘' } },
        { path: 'users', name: 'users', component: () => import('../views/Users.vue'), meta: { title: '用户管理' } },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  if (to.meta.public) return true
  if (!(await ensureMe())) return { name: 'login', query: { redirect: to.fullPath } }
  return true
})

router.afterEach((to) => {
  document.title = `${(to.meta.title as string) ?? ''} - XZJI 后台`
})
