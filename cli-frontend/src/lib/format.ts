import type { Address } from '../types'

export function shortAddress(address: string): string {
  if (address.length <= 12) return address
  return `${address.slice(0, 6)}...${address.slice(-4)}`
}

export function sameAddress(a?: string, b?: string): boolean {
  return Boolean(a && b && a.toLowerCase() === b.toLowerCase())
}

export function formatAmount(value: bigint): string {
  const whole = value / 1_000_000_000_000_000_000n
  const fraction = value % 1_000_000_000_000_000_000n
  const decimals = fraction.toString().padStart(18, '0').slice(0, 4).replace(/0+$/, '')
  return `${whole}${decimals ? `.${decimals}` : ''} ETH`
}

export function formatTimestamp(value: bigint): string {
  if (value === 0n) return '-'
  return new Date(Number(value) * 1000).toLocaleString()
}

export function normalizeAddress(value: string): Address {
  return value as Address
}
