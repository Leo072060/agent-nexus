import { formatEther, getAddress } from 'viem'

export function shortAddress(addr: string): string {
  if (!addr) return '—'
  if (addr.length < 12) return addr
  return `${addr.slice(0, 6)}…${addr.slice(-4)}`
}

/** Checksum-safe address comparison; falls back to lowercase compare. */
export function sameAddress(a?: string, b?: string): boolean {
  if (!a || !b) return false
  try {
    return getAddress(a) === getAddress(b)
  } catch {
    return a.toLowerCase() === b.toLowerCase()
  }
}

/** Hex string (e.g. bytes32 hash) compare — case-insensitive, not an address. */
export function sameHex(a?: string, b?: string): boolean {
  if (!a || !b) return false
  return a.toLowerCase() === b.toLowerCase()
}

export function formatAmount(wei: bigint): string {
  return `${formatEther(wei)} ETH`
}

export function formatTimestamp(seconds: bigint): string {
  if (!seconds || seconds === 0n) return '—'
  return new Date(Number(seconds) * 1000).toLocaleString()
}
