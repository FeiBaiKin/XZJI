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
        { path: 'comments', name: 'comments', component: () => import('../views/Comments.vue'), meta: { title: '评论管理' } },
        { path: 'link-reports', name: 'link-reports', component: () => import('../views/LinkReports.vue'), meta: { title: '失效反馈' } },
        { path: 'levels', name: 'levels', component: () => import('../views/Levels.vue'), meta: { title: '会员与套餐' } },
        { path: 'orders', name: 'orders', component: () => import('../views/Orders.vue'), meta: { title: '订单管理' } },
        { path: 'ledger', name: 'ledger', component: () => import('../views/Ledger.vue'), meta: { title: '余额流水' } },
        { path: 'settings/payment', name: 'settings-payment', component: () => import('../views/PaymentSettings.vue'), meta: { title: '支付设置' } },
        { path: 'settings/home', name: 'settings-home', component: () => import('../views/HomeSettings.vue'), meta: { title: '首页设置' } },
        { path: 'settings/site', name: 'settings-site', component: () => import('../views/SiteSettings.vue'), meta: { title: '站点设置' } },
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
