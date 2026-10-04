<script setup lang="ts">
import type { Lock } from '~/utils/content'

// 未解锁时的操作按钮：登录、去评论、购买、开通会员
const props = defineProps<{ lock: Lock, loggedIn: boolean, price: number, size?: 'md' | 'lg' }>()
const emit = defineEmits<{ buy: [] }>()
const route = useRoute()
const { levelName } = useLevelMap()
const loginTo = computed(() => ({ path: '/login', query: { redirect: route.fullPath } }))
const btn = computed(() => props.size === 'lg' ? 'btn h-10 px-[22px] text-[14.5px]' : 'btn')
</script>

<template>
  <NuxtLink v-if="!loggedIn && lock.reason !== 'paid'" :to="loginTo" :class="btn" class="bg-jb-blue">登录</NuxtLink>
  <a v-if="lock.reason === 'comment' && loggedIn" href="#comments" :class="btn" class="bg-jb-blue">去评论</a>
  <template v-if="lock.reason === 'paid'">
    <button v-if="price > 0" type="button" :class="btn" class="bg-jb-red" @click="emit('buy')">立即购买 ¥{{ yuan(price) }}</button>
    <NuxtLink v-if="lock.rank > 0" to="/vip" :class="btn" class="bg-jb-vip text-vip">开通 {{ levelName(lock.rank) }}，免费{{ size === 'lg' ? '下载' : '查看' }}</NuxtLink>
  </template>
  <NuxtLink v-if="lock.reason === 'member'" to="/vip" :class="btn" class="bg-jb-vip text-vip">开通 {{ levelName(lock.rank) }}</NuxtLink>
</template>
