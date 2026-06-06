import { statusInfo } from '../lib/status'

export function StatusChip({ status }: { status: number }) {
  const info = statusInfo(status)
  return (
    <span
      className="chip"
      style={{ backgroundColor: `${info.color}22`, color: info.color, borderColor: `${info.color}66` }}
    >
      {info.label}
    </span>
  )
}
