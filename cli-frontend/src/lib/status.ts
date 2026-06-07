export interface StatusInfo {
  label: string
  tone: string
}

export const STATUS_DISPUTED = 5
export const STATUS_RESOLVED_SELLER = 9
export const STATUS_RESOLVED_BUYER = 10
export const STATUS_DISPUTE_TIMEOUT_SPLIT = 11

const ORDER_STATUS: Record<number, StatusInfo> = {
  0: { label: '无', tone: 'muted' },
  1: { label: '待卖家确认', tone: 'pending' },
  2: { label: '待验证者确认', tone: 'pending' },
  3: { label: '已创建', tone: 'active' },
  4: { label: '已交付待验收', tone: 'active' },
  5: { label: '争议中', tone: 'danger' },
  6: { label: '已放款', tone: 'success' },
  7: { label: '确认超时退款', tone: 'muted' },
  8: { label: '交付超时退款', tone: 'muted' },
  9: { label: '裁决给卖家', tone: 'success' },
  10: { label: '裁决给买家', tone: 'warning' },
  11: { label: '争议超时平分', tone: 'split' },
}

export function statusInfo(status: number): StatusInfo {
  return ORDER_STATUS[status] ?? { label: `未知(${status})`, tone: 'muted' }
}

export function isDisputeRelated(status: number): boolean {
  return status === STATUS_DISPUTED || status === STATUS_RESOLVED_SELLER || status === STATUS_RESOLVED_BUYER || status === STATUS_DISPUTE_TIMEOUT_SPLIT
}
