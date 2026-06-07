import type { Address } from './types'

function required(name: string): string {
  const value = import.meta.env[name]
  if (!value || String(value).trim() === '') {
    throw new Error(`${name} is required`)
  }
  return String(value).trim()
}

export const config = {
  rpcUrl: required('VITE_RPC_URL'),
  marketAddress: required('VITE_MARKET_ADDRESS') as Address,
  pollMs: Number(import.meta.env.VITE_POLL_MS ?? 8000),
}
