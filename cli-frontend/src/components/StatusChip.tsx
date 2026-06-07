import { statusInfo } from '../lib/status'

export function StatusChip({ status }: { status: number }) {
  const info = statusInfo(status)
  return <span className={`status ${info.tone}`}>{info.label}</span>
}
