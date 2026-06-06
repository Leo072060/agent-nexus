import { formatAmount, formatTimestamp } from '../lib/format'
import { StatusChip } from './StatusChip'
import { DecisionProcess } from './DecisionProcess'
import type { OrderRow } from '../types'

export function OrderDetailDrawer({ row, onClose }: { row: OrderRow; onClose: () => void }) {
  const showDispute = row.isDisputed || row.isResolved

  return (
    <aside className="drawer">
      <div className="drawer-head">
        <h3>订单 #{row.id}</h3>
        <button type="button" className="close" onClick={onClose}>
          ✕
        </button>
      </div>

      <dl className="kv">
        <dt>买家</dt>
        <dd className="mono">{row.buyer}</dd>
        <dt>卖家</dt>
        <dd className="mono">{row.seller}</dd>
        <dt>验证者</dt>
        <dd className="mono">
          {row.validator}
          {row.isMine && <span className="badge-me">我</span>}
        </dd>
        <dt>金额</dt>
        <dd>{formatAmount(row.amount)}</dd>
        <dt>验证者费用</dt>
        <dd>{formatAmount(row.validatorFee)}</dd>
        <dt>状态</dt>
        <dd>
          <StatusChip status={row.status} />
        </dd>
        <dt>创建时间</dt>
        <dd>{formatTimestamp(row.createdAt)}</dd>
      </dl>

      {showDispute && (
        <section className="onchain-outcome">
          <h4>链上裁决结果</h4>
          {row.isResolved ? (
            <p className="outcome-line">
              本金 <strong>{row.resolvedOutcome === 'seller' ? '放款给卖家' : '退款给买家'}</strong>
            </p>
          ) : (
            <p className="muted">争议中，等待裁决（响应截止 {formatTimestamp(row.responseDeadline)}）</p>
          )}
          <dl className="kv">
            <dt>resolutionHash</dt>
            <dd className="mono">{row.resolutionHash}</dd>
          </dl>
        </section>
      )}

      {showDispute &&
        (row.isMine ? (
          <DecisionProcess row={row} />
        ) : (
          <div className="not-mine">由其他验证者裁决，私有判决过程不可见。</div>
        ))}
    </aside>
  )
}
