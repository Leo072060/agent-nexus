import { connectWallet } from '../api/wallet'
import { shortAddress } from '../lib/format'
import type { Address } from '../types'

export function WalletBar({
  address,
  error,
  onConnect,
  onError,
}: {
  address: Address | null
  error: string | null
  onConnect: (address: Address) => void
  onError: (error: string | null) => void
}) {
  async function handleConnect() {
    try {
      onError(null)
      onConnect(await connectWallet())
    } catch (e) {
      onError(e instanceof Error ? e.message : String(e))
    }
  }

  return (
    <div className="wallet-bar">
      <div>
        <span className="label">钱包</span>
        <strong>{address ? shortAddress(address) : '未连接'}</strong>
      </div>
      <button type="button" onClick={() => void handleConnect()}>
        {address ? '切换钱包' : '连接钱包'}
      </button>
      {error && <span className="inline-error">{error}</span>}
    </div>
  )
}
