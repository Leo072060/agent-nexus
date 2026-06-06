import { config } from '../config'
import type { DisputeDetail, DisputeSummary, Me } from '../types'

const base = config.validatorApiBaseUrl

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${base}${path}`)
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`
    try {
      const body = (await res.json()) as { error?: string }
      if (body?.error) message = body.error
    } catch {
      // non-JSON error body — keep status text
    }
    throw new Error(message)
  }
  return (await res.json()) as T
}

export function fetchMe(): Promise<Me> {
  return getJSON<Me>('/agent-nexus/me')
}

export async function fetchDisputes(): Promise<DisputeSummary[]> {
  const body = await getJSON<{ disputes: DisputeSummary[] }>('/agent-nexus/disputes')
  return body.disputes ?? []
}

export function fetchDisputeDetail(orderId: number | string): Promise<DisputeDetail> {
  return getJSON<DisputeDetail>(`/agent-nexus/disputes/${orderId}`)
}
