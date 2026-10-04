import type { AdminSchemas } from '@xzji/api-client/admin'

// 订单与余额相关的显示文字，订单、流水、用户页共用
export const orderType: Record<AdminSchemas['OrderType'], string> = { post: '内容', membership: '会员', recharge: '充值' }

export const orderStatus: Record<AdminSchemas['OrderStatus'], { text: string, type: 'warning' | 'success' | 'info' | 'primary' }> = {
  pending: { text: '待支付', type: 'warning' },
  paid: { text: '已支付', type: 'success' },
  closed: { text: '已关闭', type: 'info' },
  refunded: { text: '已退款', type: 'primary' },
}

export const payMethodName: Record<'wechat' | 'alipay' | 'balance', string> = { wechat: '微信支付', alipay: '支付宝', balance: '余额' }

export const bizTypeName: Record<AdminSchemas['LedgerBizType'], string> = { recharge: '充值', purchase: '购买', refund: '退款', admin_adjust: '后台调整' }

export function signedYuan(fen: number) {
  return (fen > 0 ? '+' : fen < 0 ? '-' : '') + (Math.abs(fen) / 100).toFixed(2)
}
