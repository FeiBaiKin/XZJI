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
        { path: 'posts', name: 'posts', component: () => import('../views/Posts.vue'), meta: { title: '内容列表', menu: '/posts' } },
        { path: 'posts/new', name: 'post-new', component: () => import('../views/PostEdit.vue'), meta: { title: '新建内容', menu: '/posts' } },
        { path: 'posts/:id(\\d+)', name: 'post-edit', component: () => import('../views/PostEdit.vue'), meta: { title: '编辑内容', menu: '/posts' } },
        { path: 'categories', name: 'categories', component: () => import('../views/Categories.vue'), meta: { title: '分类管理' } },
        { path: 'tags', name: 'tags', component: () => import('../views/Tags.vue'), meta: { title: '标签管理' } },
        { path: 'settings/home', name: 'settings-home', component: () => import('../views/HomeSettings.vue'), meta: { title: '首页设置' } },
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
