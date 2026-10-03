<script setup lang="ts">
// 会员权益卡片。会员等级和价格在 M4 接入后改为读取后台配置
defineProps<{ note?: string }>()
const perks = ['会员资源免费', '付费资源折扣', '更多下载次数', '专属会员标识']
</script>

<template>
  <section class="relative overflow-hidden rounded-card bg-jb-vip p-5 text-vip shadow-card after:absolute after:-top-[30px] after:-right-[30px] after:size-[130px] after:rounded-full after:bg-[radial-gradient(circle,rgba(246,199,107,.25),transparent_70%)]">
    <h3 class="flex items-center gap-1.5 text-[17px] font-bold text-[#F6D48F]"><Icon name="crown" class="size-5" />{{ useRuntimeConfig().public.siteName }} 会员</h3>
    <p class="mt-0.5 text-[12.5px] opacity-75">{{ note ?? '开通后会员资源免费下载' }}</p>
    <ul class="mt-3.5 grid grid-cols-2 gap-2 text-[12.5px]">
      <li v-for="p in perks" :key="p" class="flex items-center gap-1.5"><Icon name="check" class="size-3.5 text-vip-gold" />{{ p }}</li>
    </ul>
    <NuxtLink to="/vip" class="btn relative z-10 mt-4 h-[38px] w-full bg-jb-gold text-[#5B3A00]">了解会员</NuxtLink>
  </section>
</template>
