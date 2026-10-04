import type { CategoryNode, MembershipLevel } from '~/utils/content'

// 分类树在页头、页脚和各列表页共用，同一次渲染只请求一次
export async function useCategories() {
  const api = useApi()
  const { data } = await useAsyncData('categories', async () => {
    const { data } = await api.GET('/categories')
    return data?.items ?? []
  }, { default: () => [] as CategoryNode[] })
  return categoryHelpers(data)
}

// 分类树已由页头加载，组件里同步读取，不再发请求
export function useCategoryMap() {
  const { data } = useNuxtData<CategoryNode[]>('categories')
  return categoryHelpers(computed(() => data.value ?? []))
}

function categoryHelpers(tree: Ref<CategoryNode[]>) {
  const flat = computed(() => tree.value.flatMap(c => [c, ...c.children]))
  return {
    tree,
    flat,
    bySlug: (slug: string) => flat.value.find(c => c.slug === slug),
    byId: (id: number) => flat.value.find(c => c.id === id),
    parentOf: (id: number) => tree.value.find(c => c.children.some(ch => ch.id === id)),
  }
}

// 页面标题统一加站点名
export function useSiteTitle(title: MaybeRefOrGetter<string | undefined>) {
  const siteName = useRuntimeConfig().public.siteName
  return computed(() => {
    const t = toValue(title)
    return t ? `${t} - ${siteName}` : siteName
  })
}

// useAsyncData 会把数据函数里抛出的错误收进 error，需要在页面里重新抛出才会显示错误页
export function throwIfError(error: Ref<unknown>) {
  if (error.value) throw createError({ ...(error.value as object), fatal: true })
}

// 接口返回 404 时显示 404 页，其他错误显示 500 页
export function notFoundIf(response: Response | undefined, message: string): never | void {
  if (response?.status === 404) throw createError({ statusCode: 404, statusMessage: message, fatal: true })
  if (!response?.ok) throw createError({ statusCode: 500, statusMessage: '服务暂时不可用，请稍后刷新重试', fatal: true })
}

// 详情页指定导航里高亮的顶级分类
export function useNavActive(to: MaybeRefOrGetter<string | undefined>) {
  const route = useRoute()
  const state = useState<{ path: string, to: string } | null>('nav:active', () => null)
  watchEffect(() => {
    const v = toValue(to)
    state.value = v ? { path: route.path, to: v } : null
  })
}

// 会员等级用于卡片角标和解锁提示，与分类一样由页头先加载
export async function useLevels() {
  const api = useApi()
  const { data } = await useAsyncData('levels', async () => {
    const { data } = await api.GET('/membership/levels')
    return data?.items ?? []
  }, { default: () => [] as MembershipLevel[] })
  return levelHelpers(data)
}

export function useLevelMap() {
  const { data } = useNuxtData<MembershipLevel[]>('levels')
  return levelHelpers(computed(() => data.value ?? []))
}

function levelHelpers(levels: Ref<MembershipLevel[]>) {
  return {
    levels,
    // 某个等级的名称；后台删掉了这个等级时退回“会员”
    levelName: (rank: number) => levels.value.find(l => l.rank === rank)?.name ?? '会员',
    // 等级不低于 rank 的所有等级，用于“VIP 免费、SVIP 免费”
    levelsFrom: (rank: number) => levels.value.filter(l => l.rank >= rank),
  }
}
