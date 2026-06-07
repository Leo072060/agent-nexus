import { useMemo, useState } from 'react'
import { MarketHeader } from './components/MarketHeader'
import { MarketLists } from './components/MarketLists'
import { OrderDetail } from './components/OrderDetail'
import { OrdersTable } from './components/OrdersTable'
import { WalletBar } from './components/WalletBar'
import { useMarketData } from './state/useMarketData'
import type { Address, OrderRow } from './types'

export default function App() {
  const data = useMarketData()
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [wallet, setWallet] = useState<Address | null>(null)
  const [walletError, setWalletError] = useState<string | null>(null)

  const selectedRow: OrderRow | null = selectedId == null ? null : data.rows.find((row) => row.id === selectedId) ?? null
  const totalEscrow = useMemo(
    () => data.rows.reduce((total, row) => total + row.amount + row.validatorFee + row.validatorBond, 0n),
    [data.rows],
  )

  return (
    <div className="app">
      <MarketHeader
        header={data.header}
        activeSellers={data.activeSellers}
        activeValidators={data.activeValidators}
        totalEscrow={totalEscrow}
      />

      <div className="topline">
        <WalletBar address={wallet} error={walletError} onConnect={setWallet} onError={setWalletError} />
        <div className="refresh">
          {data.lastUpdated && <span>更新于 {new Date(data.lastUpdated).toLocaleTimeString()}</span>}
          <button type="button" onClick={() => void data.refresh()}>
            刷新
          </button>
        </div>
      </div>

      {data.error && <div className="banner error">{data.error}</div>}
      {data.validatorErrors.length > 0 && <div className="banner warn">部分验证者服务不可用：{data.validatorErrors.length}</div>}

      {data.loading && !data.header ? (
        <div className="loading">加载中</div>
      ) : (
        <main className="layout">
          <div className="main-column">
            <OrdersTable rows={data.rows} selectedId={selectedRow?.id ?? null} onSelect={(row) => setSelectedId(row.id)} />
            <MarketLists sellers={data.sellers} validators={data.validators} />
          </div>
          {selectedRow && <OrderDetail row={selectedRow} wallet={wallet} onClose={() => setSelectedId(null)} />}
        </main>
      )}
    </div>
  )
}
