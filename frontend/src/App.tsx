import { useState } from 'react'
import { useDashboardData } from './state/useDashboardData'
import { MarketHeader } from './components/MarketHeader'
import { TransactionsTable } from './components/TransactionsTable'
import { ViewAsControl } from './components/ViewAsControl'
import { OrderDetailDrawer } from './components/OrderDetailDrawer'
import type { OrderRow } from './types'

export default function App() {
  const {
    loading,
    error,
    header,
    me,
    meError,
    rows,
    viewAs,
    setViewAs,
    marketMismatch,
    refresh,
    lastUpdated,
  } = useDashboardData()
  const [selectedId, setSelectedId] = useState<number | null>(null)

  const selectedRow: OrderRow | null =
    selectedId == null ? null : rows.find((r) => r.id === selectedId) ?? null

  return (
    <div className="app">
      <MarketHeader header={header} me={me} />

      {marketMismatch && (
        <div className="banner warn">
          ⚠ validator-service 的 market 地址与前端 VITE_MARKET_ADDRESS 不一致，私有数据可能不属于该市场。
        </div>
      )}
      {meError && (
        <div className="banner warn">
          validator-service 未连接（{meError}）。仍可查看链上公开数据，但看不到“我的判决流程”。
        </div>
      )}
      {error && <div className="banner error">读取链上数据失败：{error}</div>}

      <div className="toolbar">
        <ViewAsControl value={viewAs} onChange={setViewAs} me={me?.validatorAddress} />
        <div className="toolbar-right">
          {lastUpdated && (
            <span className="muted small">更新于 {new Date(lastUpdated).toLocaleTimeString()}</span>
          )}
          <button type="button" onClick={() => void refresh()}>
            刷新
          </button>
        </div>
      </div>

      {loading && !header ? (
        <div className="loading">加载中…</div>
      ) : (
        <div className="content">
          <TransactionsTable
            rows={rows}
            selectedId={selectedRow?.id ?? null}
            onSelect={(r) => setSelectedId(r.id)}
          />
          {selectedRow && <OrderDetailDrawer row={selectedRow} onClose={() => setSelectedId(null)} />}
        </div>
      )}
    </div>
  )
}
