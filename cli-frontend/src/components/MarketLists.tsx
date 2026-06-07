import { formatAmount, shortAddress } from '../lib/format'
import type { Seller, Validator } from '../types'

export function MarketLists({ sellers, validators }: { sellers: Seller[]; validators: Validator[] }) {
  return (
    <div className="market-lists">
      <section>
        <div className="section-head">
          <h2>卖家</h2>
          <span>{sellers.length}</span>
        </div>
        <div className="list">
          {sellers.map((seller) => (
            <div className="list-row" key={seller.address}>
              <div>
                <strong className="mono">{shortAddress(seller.address)}</strong>
                <span>{seller.active ? '活跃' : '停用'}</span>
              </div>
              <div>{formatAmount(seller.price)}</div>
            </div>
          ))}
        </div>
      </section>
      <section>
        <div className="section-head">
          <h2>验证者</h2>
          <span>{validators.length}</span>
        </div>
        <div className="list">
          {validators.map((validator) => (
            <div className="list-row" key={validator.address}>
              <div>
                <strong className="mono">{shortAddress(validator.address)}</strong>
                <span>{validator.active ? '活跃' : '停用'}</span>
              </div>
              <div>{formatAmount(validator.fee)}</div>
            </div>
          ))}
        </div>
      </section>
    </div>
  )
}
