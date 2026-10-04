<script setup lang="ts">
import type { HiddenBlockInfo, UnlockState } from '~/utils/content'

// 隐藏内容：服务端渲染时按条件显示锁定提示，浏览器端拿到解锁情况后显示内容或对应的操作
const props = defineProps<{ info: HiddenBlockInfo, state: UnlockState | null, memberFreeRank: number }>()
const emit = defineEmits<{ buy: [] }>()
const { levelName } = useLevelMap()

const block = computed(() => props.state?.hiddenBlocks.find(b => b.seq === props.info.seq))
const lock = computed(() => block.value ? (block.value.lock ?? null) : lockOf(props.info.cond, props.memberFreeRank))
const loggedIn = computed(() => !!props.state?.viewer.loggedIn)
</script>

<template>
  <div v-if="block?.unlocked" class="my-[18px] rounded-card border border-dashed border-[#B7E4D3] bg-[#F5FCF9] px-5 pt-3 pb-1">
    <p class="mb-2 inline-flex items-center gap-1 rounded bg-jb-green px-2 py-0.5 text-xs font-semibold text-white"><Icon name="check" class="size-3" />已解锁<template v-if="info.title">：{{ info.title }}</template></p>
    <div class="prose-x" v-html="block.contentHtml" />
  </div>
  <div v-else-if="lock" class="my-[18px] rounded-card border-[1.5px] border-dashed border-[#F2B8A8] bg-[#FFF8F5] px-5 py-[22px] text-center">
    <span class="mx-auto grid size-11 place-items-center rounded-full bg-jb-red text-white"><Icon name="lock" class="size-5" /></span>
    <b class="mt-2 block text-[15px] text-title">{{ info.title || '隐藏内容' }}</b>
    <p class="text-[13px] text-muted">{{ lockText(lock, levelName, { loggedIn, price: state?.price ?? 0, action: '查看这部分内容' }) }}</p>
    <div v-if="state" class="mt-3 flex flex-wrap justify-center gap-2.5">
      <LockActions :lock="lock" :logged-in="loggedIn" :price="state.canBuy ? state.price : 0" @buy="emit('buy')" />
    </div>
  </div>
</template>
