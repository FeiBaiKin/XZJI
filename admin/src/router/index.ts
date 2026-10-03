import { createRouter, createWebHistory } from 'vue-router'

export const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', name: 'dashboard', component: () => import('../views/Dashboard.vue'), meta: { title: '仪表盘' } },
  ],
})

router.afterEach((to) => {
  document.title = `${(to.meta.title as string) ?? ''} - XZJI 后台`
})
