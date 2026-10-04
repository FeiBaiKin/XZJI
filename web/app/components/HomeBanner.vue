<script setup lang="ts">
import type { PublicSchemas } from '@xzji/api-client/public'
import type { PostCard } from '~/utils/content'

type Banner = PublicSchemas['Banner']

// 横幅轮播：服务端渲染第一张，浏览器端每 5 秒切换；鼠标悬停或系统要求减少动效时不自动切换
const props = defineProps<{ banners: Banner[], icons: PostCard[] }>()
const siteName = useRuntimeConfig().public.siteName
const slides = computed<Banner[]>(() => props.banners.length ? props.banners : [{
  tag: '', title: `欢迎来到 ${siteName}`, subtitle: '整理安卓、电脑软件和建站资源，版本号写清楚，下载地址失效了就补。', buttonText: '', link: '/',
}])

const current = ref(0)
const paused = ref(false)
let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  if (slides.value.length < 2 || matchMedia('(prefers-reduced-motion: reduce)').matches) return
  timer = setInterval(() => {
    if (!paused.value) current.value = (current.value + 1) % slides.value.length
  }, 5000)
})
onBeforeUnmount(() => clearInterval(timer))
</script>

<template>
  <!-- 各张横幅叠放在同一个网格单元里，容器高度取最高的一张 -->
  <div class="relative grid min-h-[300px] overflow-hidden rounded-card max-md:min-h-[200px]" @mouseenter="paused = true" @mouseleave="paused = false">
    <NuxtLink
      v-for="(b, i) in slides"
      :key="i"
      :to="b.link"
      class="relative col-start-1 row-start-1 grid grid-cols-[minmax(0,1fr)_auto] items-center px-11 py-9 text-white transition-opacity duration-500 max-md:grid-cols-1 max-md:px-[22px] max-md:pt-6 max-md:pb-10"
      :class="[i === current ? 'opacity-100' : 'pointer-events-none opacity-0', !b.image && 'bg-[radial-gradient(circle_at_92%_110%,rgba(255,255,255,.18)_0_30%,transparent_31%),radial-gradient(circle_at_60%_-30%,rgba(255,255,255,.14)_0_28%,transparent_29%),linear-gradient(120deg,#3A6DFF_0%,#5B4CF0_55%,#8A4FE8_100%)]']"
      :aria-hidden="i !== current"
      :tabindex="i === current ? 0 : -1"
    >
      <template v-if="b.image">
        <img
          :src="b.image.url"
          :srcset="srcsetOf(b.image)"
          sizes="(max-width: 1100px) 100vw, 880px"
          alt=""
          :width="b.image.width"
          :height="b.image.height"
          :loading="i === 0 ? 'eager' : 'lazy'"
          :fetchpriority="i === 0 ? 'high' : undefined"
          class="absolute inset-0 size-full object-cover"
        >
        <span class="absolute inset-0 bg-linear-to-r from-black/55 via-black/25 to-transparent" />
      </template>
      <span class="relative">
        <span v-if="b.tag" class="inline-block rounded-full bg-white/20 px-2.5 py-[3px] text-[13px] font-semibold">{{ b.tag }}</span>
        <component :is="i === 0 ? 'h1' : 'p'" class="mt-3 text-[34px] leading-[1.3] font-extrabold tracking-[.01em] whitespace-pre-line max-md:text-2xl">{{ b.title }}</component>
        <span v-if="b.subtitle" class="mt-2.5 block max-w-[24em] text-[15px] opacity-90 max-md:text-sm">{{ b.subtitle }}</span>
        <span v-if="b.buttonText" class="btn mt-[22px] h-10 bg-white px-[22px] text-[#3F4FE8]">{{ b.buttonText }}</span>
      </span>
      <span v-if="!b.image && icons.length >= 3" class="relative mr-2 grid -rotate-[8deg] grid-cols-[repeat(3,72px)] gap-4 max-md:absolute max-md:top-[18px] max-md:-right-1.5 max-md:grid-cols-[repeat(3,48px)] max-md:gap-2.5 max-md:opacity-35" aria-hidden="true">
        <AppTile
          v-for="(p, k) in icons.slice(0, 6)"
          :key="p.id"
          :post="p"
          class="size-[72px] text-[72px] shadow-[0_14px_24px_-10px_rgba(20,10,80,.55)] outline-3 outline-white/90 max-md:size-12 max-md:text-[48px]"
          :class="(k === 1 || k === 4) && '-translate-y-[18px]'"
        />
      </span>
    </NuxtLink>
    <span v-if="slides.length > 1" class="absolute bottom-[18px] left-11 flex gap-1.5 max-md:left-[22px]">
      <button
        v-for="(_, i) in slides"
        :key="i"
        type="button"
        class="h-2 rounded-full transition-all"
        :class="i === current ? 'w-[22px] bg-white' : 'w-2 bg-white/45'"
        :aria-label="`第 ${i + 1} 张`"
        :aria-current="i === current"
        @click="current = i"
      />
    </span>
  </div>
</template>
