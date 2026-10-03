<script setup lang="ts">
const perks = [
  { icon: 'star', text: '收藏资源，换台电脑也能找到' },
  { icon: 'box', text: '买过的资源随时重新下载' },
  { icon: 'crown', text: '开通会员，2,300+ 资源免费下' },
  { icon: 'refresh', text: '链接失效一键反馈，24 小时内补' },
]
const apps = [
  { c: '#EB7419', i: 'search' }, { c: '#2D7FF0', i: 'crop' }, { c: '#1E9AB5', i: 'box' },
  { c: '#F2B321', i: 'play' }, { c: '#23A55A', i: 'camera' }, { c: '#1F9D6B', i: 'usb' },
]
defineProps<{ title: string }>()
</script>

<template>
  <div class="relative overflow-hidden bg-brand px-9 py-10 text-white max-md:hidden">
    <h2 class="text-[26px] leading-snug font-bold">{{ title }}</h2>
    <p class="mt-1.5 opacity-85">安卓、电脑软件和建站资源，版本号写清楚，失效了就补。</p>
    <ul class="mt-6 flex flex-col gap-3">
      <li v-for="p in perks" :key="p.icon" class="flex items-center gap-2.5">
        <b class="grid size-[30px] place-items-center rounded-lg bg-white/20"><Icon :name="p.icon" /></b>{{ p.text }}
      </li>
    </ul>
    <span class="absolute -right-7 -bottom-9 grid rotate-[-8deg] grid-cols-3 gap-3" aria-hidden="true">
      <span
        v-for="a in apps"
        :key="a.i"
        class="grid size-14 place-items-center rounded-[24%] text-white shadow-[0_14px_24px_-10px_rgba(20,10,80,.55)] outline-3 outline-white/90 even:-translate-y-4"
        :style="{ background: a.c }"
      ><Icon :name="a.i" class="size-[26px]" /></span>
    </span>
  </div>
</template>
