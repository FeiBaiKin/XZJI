<script setup lang="ts">
// 会员权益卡片，折扣和下载次数取自后台配置的会员等级
defineProps<{ note?: string }>()
const { levels } = useLevelMap()
const perks = computed(() => {
  const ls = levels.value
  const min = Math.min(100, ...ls.map(l => l.discountRate))
  const unlimited = ls.some(l => l.dailyDownloadLimit === 0)
  const most = Math.max(0, ...ls.map(l => l.dailyDownloadLimit))
  return [
    '会员资源免费',
    min < 100 ? `付费资源低至 ${min / 10} 折` : '付费资源折扣',
    unlimited ? '免费下载不限次数' : most ? `每天最多 ${most} 次下载` : '更多下载次数',
    '专属会员标识',
  ]
})
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
