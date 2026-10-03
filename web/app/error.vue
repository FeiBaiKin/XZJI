<script setup lang="ts">
import type { NuxtError } from '#app'

const props = defineProps<{ error: NuxtError }>()
const notFound = computed(() => props.error.statusCode === 404)
useSeoMeta({ title: useSiteTitle(() => notFound.value ? '页面不存在' : '出错了'), robots: 'noindex' })
</script>

<template>
  <NuxtLayout>
    <section class="card mx-auto mt-6 max-w-[560px] px-8 py-12 text-center max-md:px-5">
      <b class="block text-[56px] leading-none font-extrabold text-theme">{{ error.statusCode }}</b>
      <h1 class="mt-4 text-xl font-bold text-title">{{ notFound ? (error.statusMessage || '页面不存在') : '页面加载失败' }}</h1>
      <p class="mt-2 text-muted">{{ notFound ? '链接可能已经失效，可以回到首页或搜索看看。' : '服务暂时不可用，请稍后刷新重试。' }}</p>
      <div class="mt-6 flex justify-center gap-2.5">
        <button type="button" class="btn bg-jb-blue" @click="clearError({ redirect: '/' })">回到首页</button>
        <button v-if="notFound" type="button" class="btn btn-soft" @click="clearError({ redirect: '/search' })">搜索资源</button>
      </div>
    </section>
  </NuxtLayout>
</template>
