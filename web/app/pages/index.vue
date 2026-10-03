<script setup lang="ts">
// 首页从 M2 开始按设计稿实现，这里先保留服务状态，用于检查前后端是否连通
const api = useApi()
const { data: health } = await useAsyncData('health', async () => {
  const { data, error } = await api.GET('/health')
  return data ?? error ?? null
})

useSeoMeta({ title: 'XZJI' })
</script>

<template>
  <section class="card px-6 py-5">
    <h1 class="relative pl-3 text-[17px] font-bold text-title before:absolute before:inset-y-0.5 before:left-0 before:w-1 before:rounded before:bg-theme">XZJI</h1>
    <p class="mt-2 text-muted">首页将在 M2 按设计稿实现。当前可以体验注册、登录、找回密码和账号设置。</p>
    <dl v-if="health" class="mt-4 grid w-fit grid-cols-[88px_1fr] gap-x-3 gap-y-1">
      <dt class="text-muted">服务状态</dt>
      <dd :class="health.status === 'ok' ? 'font-semibold text-success' : 'font-semibold text-danger'">{{ health.status === 'ok' ? '正常' : '异常' }}</dd>
      <dt class="text-muted">数据库</dt>
      <dd :class="health.database === 'up' ? 'font-semibold text-success' : 'font-semibold text-danger'">{{ health.database === 'up' ? '正常' : '不可用' }}</dd>
      <dt class="text-muted">后端版本</dt>
      <dd>{{ health.version }}</dd>
    </dl>
  </section>
</template>
