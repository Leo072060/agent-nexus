import { useMemo, useState } from 'react'
import { createPublicClient, createWalletClient, custom, http, type Address, type Hex } from 'viem'
import { sepolia } from 'viem/chains'
import marketArtifact from '../../out/Market.sol/Market.json'
import { marketAbi } from './abi/market'

const MARKET_URI = 'ipfs://agent-nexus-demo-market'
const SEPOLIA_CHAIN_ID = 11155111

type DeployStatus = 'idle' | 'connecting' | 'wrong-chain' | 'ready' | 'deploying' | 'waiting' | 'deployed' | 'error'

function optionalEnv(name: string): string {
  const value = import.meta.env[name]
  return value ? String(value).trim() : ''
}

function shortAddress(address: string): string {
  return `${address.slice(0, 6)}...${address.slice(-4)}`
}

function explorerTx(hash: string): string {
  return `https://sepolia.etherscan.io/tx/${hash}`
}

function explorerAddress(address: string): string {
  return `https://sepolia.etherscan.io/address/${address}`
}

export default function DeployPage() {
  const rpcUrl = optionalEnv('VITE_RPC_URL') || 'https://sepolia.infura.io/v3/9aa3d95b3bc440fa88ea12eaa4456161'
  const publicClient = useMemo(() => createPublicClient({ chain: sepolia, transport: http(rpcUrl) }), [rpcUrl])
  const bytecode = marketArtifact.bytecode.object as Hex

  const [status, setStatus] = useState<DeployStatus>('idle')
  const [account, setAccount] = useState<Address | null>(null)
  const [chainId, setChainId] = useState<number | null>(null)
  const [txHash, setTxHash] = useState<Hex | null>(null)
  const [marketAddress, setMarketAddress] = useState<Address | null>(null)
  const [error, setError] = useState<string | null>(null)

  async function connect() {
    setError(null)
    setStatus('connecting')
    try {
      if (!window.ethereum) throw new Error('未检测到 MetaMask 或浏览器钱包')
      const [connected] = await window.ethereum.request<Address[]>({ method: 'eth_requestAccounts' })
      if (!connected) throw new Error('钱包未返回账户')
      const chain = await window.ethereum.request<string>({ method: 'eth_chainId' })
      const parsedChain = Number.parseInt(chain, 16)
      setAccount(connected)
      setChainId(parsedChain)
      setStatus(parsedChain === SEPOLIA_CHAIN_ID ? 'ready' : 'wrong-chain')
    } catch (err) {
      setStatus('error')
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  async function switchToSepolia() {
    setError(null)
    try {
      if (!window.ethereum) throw new Error('未检测到 MetaMask 或浏览器钱包')
      await window.ethereum.request({
        method: 'wallet_switchEthereumChain',
        params: [{ chainId: '0xaa36a7' }],
      })
      await connect()
    } catch (err) {
      setStatus('error')
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  async function deploy() {
    setError(null)
    setMarketAddress(null)
    try {
      if (!window.ethereum) throw new Error('未检测到 MetaMask 或浏览器钱包')
      if (!account) throw new Error('请先连接钱包')
      if (chainId !== SEPOLIA_CHAIN_ID) throw new Error('请先切换到 Sepolia')

      setStatus('deploying')
      const walletClient = createWalletClient({
        account,
        chain: sepolia,
        transport: custom(window.ethereum),
      })
      const hash = await walletClient.deployContract({
        abi: marketAbi,
        bytecode,
        args: [MARKET_URI],
      })
      setTxHash(hash)
      setStatus('waiting')

      const receipt = await publicClient.waitForTransactionReceipt({ hash })
      if (!receipt.contractAddress) throw new Error('交易已确认，但 receipt 没有返回合约地址')
      setMarketAddress(receipt.contractAddress)
      setStatus('deployed')
    } catch (err) {
      setStatus('error')
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  const busy = status === 'connecting' || status === 'deploying' || status === 'waiting'
  const canDeploy = Boolean(account && chainId === SEPOLIA_CHAIN_ID && !busy)

  return (
    <div className="deploy-page">
      <header className="deploy-hero">
        <div>
          <p className="eyebrow">Agent Nexus</p>
          <h1>Deploy Market to Sepolia</h1>
          <p>用 MetaMask 签名部署 Market 合约，部署者私钥不会进入终端或本地文件。</p>
        </div>
        <a className="ghost-link" href="/">
          返回市场
        </a>
      </header>

      <main className="deploy-grid">
        <section className="deploy-panel">
          <div className="section-head">
            <h2>钱包</h2>
            <span>Sepolia chain id: {SEPOLIA_CHAIN_ID}</span>
          </div>
          <div className="deploy-body">
            <div className="deploy-row">
              <span>账户</span>
              <strong className="mono">{account ? shortAddress(account) : '未连接'}</strong>
            </div>
            <div className="deploy-row">
              <span>当前网络</span>
              <strong>{chainId == null ? '未知' : chainId === SEPOLIA_CHAIN_ID ? 'Sepolia' : `Chain ${chainId}`}</strong>
            </div>
            <div className="button-row">
              <button type="button" onClick={() => void connect()} disabled={busy}>
                {account ? '重新连接' : '连接钱包'}
              </button>
              {status === 'wrong-chain' && (
                <button type="button" className="ghost" onClick={() => void switchToSepolia()}>
                  切换 Sepolia
                </button>
              )}
            </div>
          </div>
        </section>

        <section className="deploy-panel">
          <div className="section-head">
            <h2>合约</h2>
            <span>Market</span>
          </div>
          <div className="deploy-body">
            <div className="deploy-row">
              <span>Constructor</span>
              <strong className="mono">{MARKET_URI}</strong>
            </div>
            <div className="deploy-row">
              <span>RPC</span>
              <strong className="mono">{rpcUrl}</strong>
            </div>
            <button type="button" onClick={() => void deploy()} disabled={!canDeploy}>
              {status === 'deploying' ? '等待 MetaMask 确认' : status === 'waiting' ? '等待链上确认' : '部署 Market'}
            </button>
          </div>
        </section>

        <section className="deploy-panel deploy-results">
          <div className="section-head">
            <h2>结果</h2>
            <span>{status}</span>
          </div>
          <div className="deploy-body">
            {txHash && (
              <div className="deploy-row">
                <span>交易</span>
                <a className="mono" href={explorerTx(txHash)} target="_blank" rel="noreferrer">
                  {txHash}
                </a>
              </div>
            )}
            {marketAddress && (
              <div className="deploy-row">
                <span>MARKET_ADDRESS</span>
                <a className="mono" href={explorerAddress(marketAddress)} target="_blank" rel="noreferrer">
                  {marketAddress}
                </a>
              </div>
            )}
            {marketAddress && (
              <pre className="deploy-copy">{`VITE_MARKET_ADDRESS=${marketAddress}\nVALIDATOR_MARKET_ADDRESS=${marketAddress}\nSELLER_MARKET_ADDRESS=${marketAddress}`}</pre>
            )}
            {error && <div className="banner error">{error}</div>}
            {!txHash && !error && <p className="muted">连接钱包后点击部署，MetaMask 会弹出签名确认。</p>}
          </div>
        </section>
      </main>
    </div>
  )
}
