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
    public: {
      // 浏览器直连的接口地址，需与前台同一主域（环境变量 NUXT_PUBLIC_API_BASE）
      apiBase: 'http://localhost:8080/api/v1',
      // 站点名称，用于页面标题（环境变量 NUXT_PUBLIC_SITE_NAME）
      siteName: 'XZJI',
    },
  },
  routeRules: {
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
