import type { PublicSchemas } from '@xzji/api-client/public'

export type Order = PublicSchemas['Order']
export type OrderQuote = PublicSchemas['OrderQuote']
export type OrderType = PublicSchemas['OrderType']
export type PayMethod = PublicSchemas['PayMethod']
export type PayAction = PublicSchemas['PayAction']
export type MembershipQuote = PublicSchemas['MembershipQuote']
export type LevelPlans = PublicSchemas['LevelPlans']
export type MembershipPlan = PublicSchemas['MembershipPlan']

export const payMethodName: Record<string, string> = { wechat: '微信支付', alipay: '支付宝', balance: '余额' }

export const orderStatus: Record<string, { text: string, class: string }> = {
  pending: { text: '待支付', class: 'bg-[#FFF3E3] text-[#D97706]' },
  paid: { text: '已支付', class: 'bg-[#E6F8F1] text-[#0E9F78]' },
  closed: { text: '已关闭', class: 'bg-soft text-muted' },
  refunded: { text: '已退款', class: 'bg-theme-soft text-theme' },
}

export const ledgerType: Record<string, string> = { recharge: '充值', purchase: '购买', refund: '退款', admin_adjust: '调整' }

// 手机浏览器跳转渠道 H5 页面，电脑扫码或打开收银台
export function payScene(): 'pc' | 'mobile' {
  return import.meta.client && matchMedia('(max-width: 767px), (pointer: coarse)').matches ? 'mobile' : 'pc'
}

export function signedYuan(fen: number): string {
  return (fen > 0 ? '+' : fen < 0 ? '-' : '') + yuan(Math.abs(fen))
}

// 订单列表里的时间：月-日 时:分
export function shortTime(iso: string): string {
  const d = new Date(iso)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

export function dateTime(iso: string): string {
  return `${dateOnly(iso)} ${shortTime(iso).slice(6)}`
}

// 到期时间距今的天数（不足一天按一天）
export function daysLeft(iso: string, now = Date.now()): number {
  return Math.max(0, Math.ceil((new Date(iso).getTime() - now) / 86400e3))
}

export function durationText(days: number): string {
  if (days === 0) return '永久'
  if (days % 365 === 0) return `${days / 365} 年`
  if (days % 30 === 0) return `${days / 30} 个月`
  return `${days} 天`
}

// 会员购买的说明：续费顺延、升级折算
export function membershipNote(m: MembershipQuote): string {
  const until = m.expiresAt ? `${m.levelName} 到期时间 ${dateOnly(m.expiresAt)}` : `永久 ${m.levelName}`
  if (m.kind === 'renew') return `到期时间从 ${dateOnly(m.fromExpiresAt!)} 顺延到 ${dateOnly(m.expiresAt!)}`
  if (m.kind === 'upgrade') {
    if (m.fromPermanent) return `已是永久 ${m.fromLevel}，补两者永久价的差价，升级后为永久 ${m.levelName}`
    if (m.durationDays === 0) return `${m.fromLevel} 剩余 ${m.remainingDays} 天折算价值 ¥${yuan(m.remainingValue)}，已从价格中抵扣`
    return `${m.fromLevel} 剩余 ${m.remainingDays} 天已折算为 ${m.convertedDays} 天，${until}`
  }
  return m.expiresAt ? `开通后 ${dateOnly(m.expiresAt)} 到期` : '永久有效'
}

// 等级卡片的配色：第一档金色、第二档紫色，更高的用黑金
export function levelTone(index: number) {
  return [
    { head: 'bg-[linear-gradient(135deg,#F7D58B,#E0A63E)] text-[#4A2F00]', on: 'border-[#E0A63E] bg-[#FFF9EC]', price: 'text-[#A86A00]', note: 'bg-[#FFF9EC] text-[#7A5200]', em: 'text-[#A86A00]', btn: 'bg-jb-gold text-[#5B3A00]', icon: 'crown' },
    { head: 'bg-[linear-gradient(135deg,#A98BFF,#6A48E0)] text-white', on: 'border-[#7B5CF0] bg-[#F6F2FF]', price: 'text-[#6A48E0]', note: 'bg-[#F6F2FF] text-[#4B3A8C]', em: 'text-[#6A48E0]', btn: 'bg-jb-purple', icon: 'gem' },
    { head: 'bg-jb-vip text-vip', on: 'border-[#5E5F62] bg-[#F6F6F7]', price: 'text-title', note: 'bg-soft text-body', em: 'text-title', btn: 'bg-jb-vip text-vip', icon: 'gem' },
  ][Math.min(index, 2)]!
}
