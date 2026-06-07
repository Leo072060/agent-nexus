import { formatAmount, formatTimestamp, shortAddress } from '../lib/format'
import { StatusChip } from './StatusChip'
import type { OrderRow } from '../types'

export function OrdersTable({
  rows,
  selectedId,
  onSelect,
}: {
  rows: OrderRow[]
  selectedId: number | null
  onSelect: (row: OrderRow) => void
}) {
  return (
    <section className="orders-panel">
      <div className="section-head">
        <h2>订单</h2>
        <span>{rows.length}</span>
      </div>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>买家 / 卖家</th>
              <th>验证者</th>
              <th>金额</th>
              <th>质押</th>
              <th>状态</th>
              <th>截止</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.id} className={row.id === selectedId ? 'selected' : ''} onClick={() => onSelect(row)}>
                <td>#{row.id}</td>
                <td>
                  <span className="mono">{shortAddress(row.buyer)}</span>
                  <span className="muted"> / </span>
                  <span className="mono">{shortAddress(row.seller)}</span>
                </td>
                <td className="mono">{shortAddress(row.validator)}</td>
                <td>{formatAmount(row.amount + row.validatorFee)}</td>
                <td>{formatAmount(row.validatorBond)}</td>
                <td>
                  <StatusChip status={row.status} />
                </td>
                <td>{formatTimestamp(row.responseDeadline || row.deliveryDeadline || row.approvalDeadline)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  )
}
