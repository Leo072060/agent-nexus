import { config } from '../config'
import { shortAddress } from '../lib/format'
import type { MarketHeaderData, Me } from '../types'

export function MarketHeader({ header, me }: { header: MarketHeaderData | null; me: Me | null }) {
  return (
    <header className="market-header">
      <div className="market-title">
        <h1>Validator 看板</h1>
        <p className="muted mono">
          Market {shortAddress(config.marketAddress)}
          {header?.marketURI ? ` · ${header.marketURI}` : ''}
        </p>
      </div>
      <div className="stats">
        <div className="stat">
          <span className="stat-num">{header ? header.orderCount : '—'}</span>
          <span className="muted">订单总数</span>
        </div>
        <div className="stat">
          <span className="stat-val mono">{me ? shortAddress(me.validatorAddress) : '未连接'}</span>
          <span className="muted">我的验证者</span>
        </div>
      </div>
    </header>
  )
}
