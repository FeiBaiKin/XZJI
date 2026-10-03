import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 后台由后端程序在 /admin/ 下托管；开发时把接口请求代理到本地后端，保持同源
export default defineConfig({
  base: '/admin/',
  plugins: [vue()],
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: {
      '/admin-api': 'http://127.0.0.1:8080',
    },
  },
})
