import type { Address } from './types'

function clean(value: string | undefined): string {
  return (value ?? '').trim()
}

export const config = {
  rpcUrl: clean(import.meta.env.VITE_RPC_URL),
  marketAddress: clean(import.meta.env.VITE_MARKET_ADDRESS) as Address,
  validatorApiBaseUrl: (clean(import.meta.env.VITE_VALIDATOR_API_BASE_URL) || 'http://localhost:8082').replace(/\/+$/, ''),
  defaultViewAs: clean(import.meta.env.VITE_DEFAULT_VIEW_AS),
  pollMs: Number(clean(import.meta.env.VITE_POLL_MS) || '15000') || 0,
}

const ZERO_ADDRESS = '0x0000000000000000000000000000000000000000'

/** Returns a human-readable reason if required config is missing/invalid, else null. */
export function configError(): string | null {
  if (!config.rpcUrl) return 'VITE_RPC_URL 未配置'
  if (!config.marketAddress || config.marketAddress === ZERO_ADDRESS) return 'VITE_MARKET_ADDRESS 未配置'
  if (!/^0x[0-9a-fA-F]{40}$/.test(config.marketAddress)) return 'VITE_MARKET_ADDRESS 不是合法的地址'
  return null
}
