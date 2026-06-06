import { formatAmount, shortAddress } from '../lib/format'
import { StatusChip } from './StatusChip'
import type { OrderRow } from '../types'

export function TransactionsTable({
  rows,
  selectedId,
  onSelect,
}: {
  rows: OrderRow[]
  selectedId: number | null
  onSelect: (row: OrderRow) => void
}) {
  if (rows.length === 0) {
    return <div className="empty">该市场暂无订单</div>
  }

  return (
    <table className="tx-table">
      <thead>
        <tr>
          <th>#</th>
          <th>买家 → 卖家</th>
          <th>验证者</th>
          <th>金额</th>
          <th>状态</th>
          <th>争议</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => (
          <tr
            key={row.id}
            className={selectedId === row.id ? 'selected' : ''}
            onClick={() => onSelect(row)}
          >
            <td>{row.id}</td>
            <td className="mono">
              {shortAddress(row.buyer)} <span className="arrow">→</span> {shortAddress(row.seller)}
            </td>
            <td className="mono">
              {shortAddress(row.validator)}
              {row.isMine && <span className="badge-me">我</span>}
            </td>
            <td>{formatAmount(row.amount)}</td>
            <td>
              <StatusChip status={row.status} />
            </td>
            <td>
              {row.isResolved ? (
                <span className="tag tag-resolved">已裁决</span>
              ) : row.isDisputed ? (
                <span className="tag tag-disputed">争议中</span>
              ) : (
                <span className="muted">—</span>
              )}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
