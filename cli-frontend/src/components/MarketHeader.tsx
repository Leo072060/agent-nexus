import { formatAmount, shortAddress } from '../lib/format'
import type { MarketHeaderData } from '../types'

export function MarketHeader({
  header,
  activeSellers,
  activeValidators,
  totalEscrow,
}: {
  header: MarketHeaderData | null
  activeSellers: number
  activeValidators: number
  totalEscrow: bigint
}) {
  return (
    <header className="market-header">
      <div>
        <p className="eyebrow">Agent Nexus</p>
        <h1>{header?.marketURI || 'Market'}</h1>
      </div>
      <div className="stats">
        <div>
          <span>订单</span>
          <strong>{header?.orderCount ?? 0}</strong>
        </div>
        <div>
          <span>卖家</span>
          <strong>{activeSellers}</strong>
        </div>
        <div>
          <span>验证者</span>
          <strong>{activeValidators}</strong>
        </div>
        <div>
          <span>锁定资金</span>
          <strong>{formatAmount(totalEscrow)}</strong>
        </div>
        <div className="owner">
          <span>Owner</span>
          <strong>{header?.owner ? shortAddress(header.owner) : '-'}</strong>
        </div>
      </div>
    </header>
  )
}
