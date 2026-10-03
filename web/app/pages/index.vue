<script setup lang="ts">
import type { PublicSchemas } from '@xzji/api-client/public'

type Health = PublicSchemas['Health']

const api = useApi()

// 服务端渲染阶段请求，结果随 HTML 一起输出
const { data: ssr, error: ssrError } = await useAsyncData('health', async () => {
  const { data, error } = await api.GET('/health')
  return data ?? error ?? null
})

// 页面加载后浏览器再直连一次，验证跨域配置
const browser = ref<Health | null>(null)
const browserError = ref('')
onMounted(async () => {
  try {
    const { data, error } = await api.GET('/health')
    browser.value = data ?? error ?? null
  } catch {
    browserError.value = '浏览器无法连接接口，请检查后端是否启动，以及 cors.allow_origins 是否包含当前地址'
  }
})

useSeoMeta({ title: 'XZJI 前台骨架' })
</script>

<template>
  <main class="m0">
    <h1>XZJI 前台骨架</h1>
    <p class="note">这个页面只用于验证 M0：服务端渲染和浏览器都能通过生成的客户端调用后端接口。正式页面从 M2 开始按设计稿实现。</p>

    <section>
      <h2>服务端渲染时请求</h2>
      <p v-if="ssrError" class="bad">请求失败：{{ ssrError.message }}</p>
      <dl v-else-if="ssr">
        <dt>服务状态</dt><dd :class="ssr.status">{{ ssr.status }}</dd>
        <dt>数据库</dt><dd :class="ssr.database">{{ ssr.database }}</dd>
        <dt>后端版本</dt><dd>{{ ssr.version }}</dd>
        <dt>响应时间</dt><dd>{{ ssr.time }}</dd>
      </dl>
    </section>

    <section>
      <h2>浏览器直连请求</h2>
      <p v-if="browserError" class="bad">{{ browserError }}</p>
      <dl v-else-if="browser">
        <dt>服务状态</dt><dd :class="browser.status">{{ browser.status }}</dd>
        <dt>数据库</dt><dd :class="browser.database">{{ browser.database }}</dd>
        <dt>响应时间</dt><dd>{{ browser.time }}</dd>
      </dl>
      <p v-else class="note">请求中…</p>
    </section>
  </main>
</template>

<style scoped>
.m0 { max-width: 720px; margin: 48px auto; padding: 0 20px; font: 14px/1.7 "PingFang SC", "Microsoft YaHei", system-ui, sans-serif; color: #4E5358; }
h1 { font-size: 22px; color: #262A31; }
h2 { font-size: 16px; color: #262A31; margin: 24px 0 8px; padding-left: 10px; border-left: 4px solid #2F6BFF; }
.note { color: #8A9099; }
section { background: #fff; border-radius: 10px; box-shadow: 0 0 12px rgba(116, 116, 116, .08); padding: 4px 20px 16px; margin-top: 16px; }
dl { display: grid; grid-template-columns: 100px 1fr; gap: 6px 12px; }
dt { color: #8A9099; }
dd { margin: 0; color: #262A31; }
.ok, .up { color: #12B087; font-weight: 600; }
.degraded, .down, .bad { color: #FF4D3D; font-weight: 600; }
</style>
