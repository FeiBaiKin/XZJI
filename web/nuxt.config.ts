import tailwindcss from '@tailwindcss/vite'

export default defineNuxtConfig({
  compatibilityDate: '2026-10-01',
  devtools: { enabled: false },
  css: ['~/assets/css/main.css'],
  vite: {
    plugins: [tailwindcss()],
  },
  runtimeConfig: {
    // 服务端渲染时调用接口的地址，分机部署时填后端内网地址（环境变量 NUXT_API_INTERNAL_BASE）
    apiInternalBase: 'http://127.0.0.1:8080/api/v1',
    // 后端清空缓存的密钥（环境变量 NUXT_PURGE_SECRET），要和后端 frontend.purge_secret 一致
    purgeSecret: '',
    public: {
      // 浏览器直连的接口地址，需与前台同一主域（环境变量 NUXT_PUBLIC_API_BASE）
      apiBase: 'http://localhost:8080/api/v1',
      // 站点名称，用于页面标题（环境变量 NUXT_PUBLIC_SITE_NAME）
      siteName: 'XZJI',
      // 站点对外地址，用于 canonical、og:url 和 sitemap（环境变量 NUXT_PUBLIC_SITE_URL），末尾不带 /
      siteUrl: 'http://localhost:3000',
    },
  },
  routeRules: {
    // 公开页面开路由级缓存：内容变更时后端会通知清空，这里的 10 分钟只是兜底过期
    '/': { cache: { maxAge: 600 } },
    // 内容详情 /{id}.html
    '/*.html': { cache: { maxAge: 600 } },
    '/category/**': { cache: { maxAge: 600 } },
    '/tag/**': { cache: { maxAge: 600 } },
    '/vip': { cache: { maxAge: 600 } },
    '/terms': { cache: { maxAge: 600 } },
    '/privacy': { cache: { maxAge: 600 } },
    // 搜索结果跟着查询串变化，短缓存就够
    '/search': { cache: { maxAge: 60 } },
    // 用户中心、登录相关页面不需要 SEO，只在浏览器端渲染
    '/user/**': { ssr: false },
    '/pay/**': { ssr: false },
    '/login': { ssr: false },
    '/forgot': { ssr: false },
  },
  app: {
    head: {
      htmlAttrs: { lang: 'zh-CN' },
      title: 'XZJI',
    },
  },
  typescript: { strict: true },
})
