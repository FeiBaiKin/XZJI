import type { PublicSchemas } from '@xzji/api-client/public'

type Overview = PublicSchemas['AccountOverview']

// 用户中心概览：余额、已购数量、今日下载；用户中心各页共用，支付后调用 refresh 更新
export function useAccount() {
  const overview = useState<Overview | null>('account:overview', () => null)
  async function refresh() {
    const { data } = await useApi().GET('/me/overview')
    overview.value = data ?? null
  }
  return { overview, refresh }
}
