<script setup lang="ts">
import { errorMessage, type PublicSchemas } from '@xzji/api-client/public'

type Comment = PublicSchemas['Comment']

// 评论区：在浏览器端加载，登录用户能看到自己待审核的评论
const props = defineProps<{ postId: number, count: number }>()
const emit = defineEmits<{ changed: [] }>()
const route = useRoute()
const toast = useToast()
const { user, ready } = useAuth()

const items = ref<Comment[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)
const pageSize = 10

async function load(p = 1) {
  loading.value = true
  const { data } = await useApi().GET('/posts/{id}/comments', { params: { path: { id: props.postId }, query: { page: p, pageSize } } })
  loading.value = false
  if (!data) return
  items.value = p === 1 ? data.items : [...items.value, ...data.items]
  total.value = data.total
  page.value = p
}
onMounted(() => load())

const content = ref('')
const replyTo = ref<Comment | null>(null)
const replyRoot = ref<number>(0)
const replyContent = ref('')
const sending = ref(false)

function startReply(c: Comment, rootId: number) {
  replyTo.value = c
  replyRoot.value = rootId
  replyContent.value = ''
}

async function send(text: string, target: Comment | null) {
  if (text.trim().length < 2) return toast.error('评论至少 2 个字')
  sending.value = true
  const { data, error } = await useApi().POST('/posts/{id}/comments', {
    params: { path: { id: props.postId } },
    body: { content: text, replyToId: target?.id },
  })
  sending.value = false
  if (!data) return toast.error(errorMessage(error))
  toast.ok(data.status === 'pending' ? '评论已提交，审核通过后显示' : '评论已发表')
  if (target) {
    replyTo.value = null
    replyContent.value = ''
  } else {
    content.value = ''
  }
  await load()
  emit('changed')
}

const loginTo = computed(() => ({ path: '/login', query: { redirect: route.fullPath } }))
</script>

<template>
  <section id="comments" class="card mt-5 scroll-mt-20 px-[18px] pt-4 pb-1.5">
    <h2 class="sec-title mb-3">评论 {{ count || '' }}</h2>
    <div class="flex gap-3 pb-2">
      <span class="grid size-[38px] shrink-0 place-items-center rounded-full bg-soft text-[#B7BDC6]"><Icon name="user" class="size-5" /></span>
      <ClientOnly>
        <form v-if="user" class="flex-1" @submit.prevent="send(content, null)">
          <textarea v-model="content" rows="3" maxlength="1000" placeholder="说说你的看法。审核通过后可以查看“评论可见”的内容" class="field-input h-auto resize-y py-2" />
          <div class="mt-2 flex items-center justify-end gap-3">
            <span class="text-xs text-muted">{{ content.length }} / 1000</span>
            <button type="submit" class="btn bg-jb-blue" :disabled="sending">发表评论</button>
          </div>
        </form>
        <div v-else-if="ready" class="flex h-[76px] flex-1 items-center justify-between gap-3 rounded-lg bg-soft px-3 text-[13.5px] text-muted">
          登录后发表评论，审核通过后可以查看“评论可见”的内容
          <NuxtLink :to="loginTo" class="btn shrink-0 bg-jb-blue">登录</NuxtLink>
        </div>
        <template #fallback><div class="h-[76px] flex-1 rounded-lg bg-soft" /></template>
      </ClientOnly>
    </div>

    <p v-if="!loading && !items.length" class="py-6 text-center text-[13px] text-muted">还没有评论，来说第一句吧</p>
    <CommentItem v-for="c in items" :key="c.id" :comment="c" @reply="startReply($event, c.id)">
      <div v-if="c.replies.length" class="mt-2 rounded-lg bg-soft px-3">
        <CommentItem v-for="r in c.replies" :key="r.id" :comment="r" reply @reply="startReply($event, c.id)" />
      </div>
      <form v-if="replyTo && replyRoot === c.id" class="mt-2" @submit.prevent="send(replyContent, replyTo)">
        <textarea v-model="replyContent" rows="2" maxlength="1000" :placeholder="`回复 @${replyTo.author.nickname}`" class="field-input h-auto resize-y py-2" />
        <div class="mt-2 flex justify-end gap-2">
          <button type="button" class="btn btn-soft" @click="replyTo = null">取消</button>
          <button v-if="user" type="submit" class="btn bg-jb-blue" :disabled="sending">回复</button>
          <NuxtLink v-else :to="loginTo" class="btn bg-jb-blue">登录后回复</NuxtLink>
        </div>
      </form>
    </CommentItem>
    <div v-if="items.length < total" class="py-3 text-center">
      <button type="button" class="btn btn-soft" :disabled="loading" @click="load(page + 1)">查看更多评论</button>
    </div>
  </section>
</template>
