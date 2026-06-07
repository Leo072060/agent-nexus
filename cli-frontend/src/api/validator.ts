import type { Address, DisputeDetail, DisputeSummary } from '../types'

export interface NonceResponse {
  address: Address
  orderId: string
  nonce: string
  message: string
  expiresAt: string
}

function endpoint(baseURL: string, path: string): string {
  return `${baseURL.replace(/\/+$/, '')}${path}`
}

async function getJSON<T>(url: string): Promise<T> {
  const response = await fetch(url)
  if (!response.ok) {
    const body = await response.text()
    throw new Error(`${response.status} ${body}`)
  }
  return response.json() as Promise<T>
}

export async function fetchDisputeSummaries(baseURL: string): Promise<DisputeSummary[]> {
  const body = await getJSON<{ disputes: DisputeSummary[] }>(endpoint(baseURL, '/agent-nexus/disputes'))
  return body.disputes
}

export async function fetchNonce(baseURL: string, address: Address, orderId: number): Promise<NonceResponse> {
  const params = new URLSearchParams({ address, orderId: String(orderId) })
  return getJSON<NonceResponse>(endpoint(baseURL, `/agent-nexus/auth/nonce?${params.toString()}`))
}

export async function fetchDisputeDetail(baseURL: string, orderId: number, address: Address, nonce: string, signature: string): Promise<DisputeDetail> {
  const params = new URLSearchParams({ address, nonce, signature })
  return getJSON<DisputeDetail>(endpoint(baseURL, `/agent-nexus/disputes/${orderId}?${params.toString()}`))
}
