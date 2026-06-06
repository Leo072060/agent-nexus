import type { ResolvedOutcome } from '../types'

export interface StatusInfo {
  label: string
  color: string
}

// OrderStatus enum (MarketStorage.sol) → 中文标签 + 颜色
const ORDER_STATUS: Record<number, StatusInfo> = {
  0: { label: '无', color: '#9ca3af' },
  1: { label: '待卖家确认', color: '#f59e0b' },
  2: { label: '待验证者确认', color: '#f59e0b' },
  3: { label: '已创建', color: '#3b82f6' },
  4: { label: '已交付待验收', color: '#6366f1' },
  5: { label: '争议中', color: '#ef4444' },
  6: { label: '已放款', color: '#10b981' },
  7: { label: '确认超时退款', color: '#9ca3af' },
  8: { label: '交付超时退款', color: '#9ca3af' },
  9: { label: '裁决给卖家', color: '#10b981' },
  10: { label: '裁决给买家', color: '#f97316' },
}

export const STATUS_DISPUTED = 5
export const STATUS_RESOLVED_SELLER = 9
export const STATUS_RESOLVED_BUYER = 10

export function statusInfo(status: number): StatusInfo {
  return ORDER_STATUS[status] ?? { label: `未知(${status})`, color: '#9ca3af' }
}

export function isDisputedStatus(status: number): boolean {
  return status === STATUS_DISPUTED
}

export function isResolvedStatus(status: number): boolean {
  return status === STATUS_RESOLVED_SELLER || status === STATUS_RESOLVED_BUYER
}

export function resolvedOutcome(status: number): ResolvedOutcome {
  if (status === STATUS_RESOLVED_SELLER) return 'seller'
  if (status === STATUS_RESOLVED_BUYER) return 'buyer'
  return null
}
