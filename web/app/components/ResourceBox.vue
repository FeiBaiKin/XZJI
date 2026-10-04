<script setup lang="ts">
import { errorCode, errorMessage, type PublicSchemas } from '@xzji/api-client/public'
import type { PostDetail, UnlockState } from '~/utils/content'

type Link = PublicSchemas['LinkInfo']

// 资源的下载区：价格、会员权益、解锁操作和下载地址。服务端渲染时按条件显示锁定状态，不含任何地址
const props = defineProps<{ post: PostDetail, state: UnlockState | null }>()
const emit = defineEmits<{ buy: [] }>()
const route = useRoute()
const toast = useToast()
const { levelName, levelsFrom } = useLevelMap()

const res = computed(() => props.post.resource!)
const cond = computed(() => res.value.downloadCond)
const download = computed(() => props.state?.download)
const lock = computed(() => download.value ? (download.value.lock ?? null) : lockOf(cond.value, props.post.memberFreeRank))
const unlocked = computed(() => !!download.value?.unlocked)
const loggedIn = computed(() => !!props.state?.viewer.loggedIn)

const label = computed(() => {
  if (cond.value === 'public') return { text: '免费资源', class: 'bg-jb-green' }
  if (cond.value === 'login' || cond.value === 'comment') return { text: cond.value === 'login' ? '登录下载' : '评论下载', class: 'bg-jb-blue' }
  if (cond.value === 'paid' && props.post.price > 0) return { text: '付费资源', class: 'bg-jb-red' }
  return { text: '会员资源', class: 'bg-jb-vip text-vip' }
})
// 可以免费下载的会员等级
const freeLevels = computed(() => {
  const rank = cond.value === 'paid' ? props.post.memberFreeRank : memberRankOf(cond.value)
  return rank > 0 ? levelsFrom(rank) : []
})
const showPrice = computed(() => cond.value === 'paid' && props.post.price > 0)
// 会员折扣价：只有当前用户的价格低于原价时显示
const memberPrice = computed(() => props.state?.canBuy && props.state.price < props.post.price ? props.state.price : 0)
const dailyTip = computed(() => {
  const d = download.value
  const limit = props.state?.viewer.dailyLimit ?? 0
  return d?.memberFree && limit > 0 ? `今天已用 ${d.usedToday} / ${limit} 次会员免费下载` : ''
})

const diskColors: Record<string, string> = { 百度: '#E03131', 夸克: '#6C5CE7', 蓝奏: '#2F80ED', 阿里: '#FF6A00', 123: '#12B087', 迅雷: '#2D7FF0', 天翼: '#E8A317' }
const diskColor = (name: string) => Object.entries(diskColors).find(([k]) => name.includes(k))?.[1] ?? '#2F6BFF'

const revealed = ref<(PublicSchemas['DownloadLink'] & { name: string }) | null>(null)
const showLink = computed({ get: () => !!revealed.value, set: (v) => { if (!v) revealed.value = null } })
const loadingId = ref(0)

async function open(link: Link) {
  if (!unlocked.value) {
    if (lock.value) toast.error(lockText(lock.value, levelName, { loggedIn: loggedIn.value, price: props.state?.price ?? 0, action: '下载' }))
    return
  }
  loadingId.value = link.id
  const { data, error } = await useApi().POST('/posts/{id}/links/{linkId}/download', { params: { path: { id: props.post.id, linkId: link.id } } })
  loadingId.value = 0
  if (!data) {
    if (errorCode(error) === 'LOGIN_REQUIRED') return navigateTo({ path: '/login', query: { redirect: route.fullPath } })
    return toast.error(errorMessage(error))
  }
  revealed.value = { ...data, name: link.name }
}

async function copy(text: string, what: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.ok(`${what}已复制`)
  } catch {
    toast.error('复制失败，请手动选择复制')
  }
}

// 失效反馈
const reportOpen = ref(false)
const reportLink = ref<number>(0)
const reason = ref('')
const reporting = ref(false)
function openReport() {
  if (!loggedIn.value) return navigateTo({ path: '/login', query: { redirect: route.fullPath } })
  reportLink.value = res.value.links[0]?.id ?? 0
  reason.value = ''
  reportOpen.value = true
}
async function submitReport() {
  reporting.value = true
  const { error, response } = await useApi().POST('/posts/{id}/links/{linkId}/reports', {
    params: { path: { id: props.post.id, linkId: reportLink.value } },
    body: { reason: reason.value.trim() || undefined },
  })
  reporting.value = false
  if (!response.ok) return toast.error(errorMessage(error))
  reportOpen.value = false
  toast.ok('已收到反馈，会尽快补上')
}
</script>

<template>
  <div class="relative mt-5 grid grid-cols-[260px_minmax(0,1fr)] gap-[22px] rounded-card border border-line bg-soft p-[18px] max-md:grid-cols-1 max-md:p-3.5">
    <span class="absolute -top-px -left-px z-10 inline-flex items-center gap-1 rounded-tl-card rounded-br-card px-2.5 py-1 text-xs font-semibold text-white" :class="label.class">
      <Icon name="down" class="size-[13px]" />{{ label.text }}
    </span>
    <PostCover :post="post" :tile="84" eager class="rounded-lg" />
    <div class="min-w-0">
      <b class="block text-[17px] leading-normal text-title">{{ post.title }}<em v-if="res.version" class="ml-1.5 font-medium not-italic text-theme">v{{ res.version }}</em></b>
      <p v-if="post.summary" class="mt-0.5 line-clamp-2 text-[13px] text-muted">{{ post.summary }}</p>
      <div v-if="showPrice" class="mt-2.5 flex items-baseline gap-2 text-danger">
        <small class="text-[15px] font-bold">¥</small>
        <strong class="-ml-1.5 text-[32px] leading-none font-extrabold">{{ yuan(memberPrice || post.price) }}</strong>
        <del v-if="memberPrice" class="text-[13px] text-muted">¥{{ yuan(post.price) }}</del>
        <span v-if="memberPrice" class="self-center rounded bg-jb-red px-2 py-0.5 text-xs font-semibold text-white">会员价</span>
      </div>
      <div v-if="freeLevels.length" class="mt-2.5 flex flex-wrap gap-2">
        <span v-for="(l, i) in freeLevels" :key="l.id" class="inline-flex items-center gap-1 rounded-[5px] px-2.5 py-1 text-[12.5px] font-semibold" :class="i === 0 ? 'bg-[#FFF4DC] text-[#A86A00]' : 'bg-[#F1EBFF] text-[#6A48E0]'">
          <Icon :name="i === 0 ? 'crown' : 'gem'" class="size-3.5" />{{ l.name }} 免费
        </span>
      </div>
      <div class="mt-3.5 flex flex-wrap items-center gap-2.5">
        <template v-if="unlocked">
          <span class="inline-flex items-center gap-1 text-sm font-semibold text-success"><Icon name="check" />{{ state?.viewer.privileged ? '你是管理员，可以直接下载' : '已解锁，点击下方网盘获取地址' }}</span>
        </template>
        <template v-else-if="lock">
          <LockActions v-if="state" :lock="lock" :logged-in="loggedIn" :price="state.canBuy ? state.price : 0" size="lg" @buy="emit('buy')" />
          <span v-else class="text-[13px] text-muted">{{ lockText(lock, levelName, { loggedIn: false, price: post.price, action: '下载' }) }}</span>
        </template>
      </div>
      <p v-if="state && lock && !unlocked" class="mt-2 text-[12.5px] text-muted">{{ lockText(lock, levelName, { loggedIn, price: state.price, action: '下载' }) }}</p>
      <p v-if="dailyTip" class="mt-2 text-[12.5px] text-muted">{{ dailyTip }}</p>
    </div>
  </div>

  <div v-if="res.links.length" class="mt-3.5 flex flex-wrap gap-2.5">
    <button
      v-for="l in res.links"
      :key="l.id"
      type="button"
      class="inline-flex h-[38px] items-center gap-2 rounded-md border border-line bg-white pr-3.5 pl-1.5 text-[13.5px] transition hover:border-theme disabled:opacity-60"
      :class="unlocked ? 'text-title' : 'text-muted'"
      :disabled="loadingId === l.id"
      @click="open(l)"
    >
      <i class="grid size-[26px] place-items-center rounded-md text-[13px] font-bold not-italic text-white" :style="{ background: diskColor(l.name) }">{{ l.name.slice(0, 1) }}</i>
      {{ l.name }}
      <span v-if="l.status === 'broken'" class="rounded bg-danger-soft px-1.5 text-xs text-danger">已失效</span>
      <Icon :name="unlocked ? 'down' : 'lock'" class="size-3.5" />
    </button>
    <button type="button" class="ml-auto inline-flex items-center gap-1 self-center text-[12.5px] text-muted hover:text-danger" @click="openReport">
      <Icon name="flag" class="size-[13px]" />链接失效反馈
    </button>
  </div>

  <AppModal v-model="showLink" :title="revealed ? `${revealed.name}下载地址` : '下载地址'">
    <template v-if="revealed">
      <p v-if="revealed.status === 'broken'" class="mt-3 rounded-md bg-danger-soft px-3 py-2 text-[13px] text-danger">这个地址已被标记为失效，可以试试其他网盘，或点“链接失效反馈”。</p>
      <dl class="mt-4 grid grid-cols-[72px_minmax(0,1fr)_auto] items-center gap-x-3 gap-y-3 text-sm">
        <dt class="text-muted">地址</dt>
        <dd class="truncate text-title" :title="revealed.url">{{ revealed.url }}</dd>
        <button type="button" class="text-theme" @click="copy(revealed.url, '地址')">复制</button>
        <template v-if="revealed.extractCode">
          <dt class="text-muted">提取码</dt>
          <dd class="font-semibold tracking-wider text-title">{{ revealed.extractCode }}</dd>
          <button type="button" class="text-theme" @click="copy(revealed.extractCode, '提取码')">复制</button>
        </template>
        <template v-if="revealed.unzipPassword">
          <dt class="text-muted">解压密码</dt>
          <dd class="font-semibold text-title">{{ revealed.unzipPassword }}</dd>
          <button type="button" class="text-theme" @click="copy(revealed.unzipPassword, '解压密码')">复制</button>
        </template>
      </dl>
      <a :href="revealed.url" target="_blank" rel="noopener noreferrer nofollow" class="btn mt-5 h-11 w-full bg-jb-blue text-[15px]">打开下载页面</a>
    </template>
  </AppModal>

  <AppModal v-model="reportOpen" title="反馈下载地址失效">
    <form class="mt-4 flex flex-col gap-3" @submit.prevent="submitReport">
      <label class="field-label" for="report-link">哪个地址打不开</label>
      <select id="report-link" v-model="reportLink" class="field-input">
        <option v-for="l in res.links" :key="l.id" :value="l.id">{{ l.name }}</option>
      </select>
      <label class="field-label" for="report-reason">补充说明（可以不填）</label>
      <textarea id="report-reason" v-model="reason" maxlength="200" rows="3" placeholder="比如：提示分享已取消、提取码不对" class="field-input h-auto py-2" />
      <button type="submit" class="btn mt-1 h-11 bg-jb-red text-[15px]" :disabled="reporting">提交反馈</button>
    </form>
  </AppModal>
</template>
