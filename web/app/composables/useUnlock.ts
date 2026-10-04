import type { UnlockState } from '~/utils/content'

// 当前用户在一篇内容上的解锁情况：只在浏览器端请求，服务端渲染的页面不带身份
export function useUnlock(postId: number) {
  const state = ref<UnlockState | null>(null)
  async function refresh() {
    const { data } = await useApi().GET('/posts/{id}/unlocked', { params: { path: { id: postId } } })
    state.value = data ?? null
  }
  onMounted(refresh)
  return { state, refresh }
}
