import { marketAbi } from '../abi/market'
import { config } from '../config'
import { publicClient } from './client'
import type { Address, MarketHeaderData, OnChainOrder } from '../types'

const market = { address: config.marketAddress, abi: marketAbi } as const

interface RawOrder {
  buyer: Address
  seller: Address
  validator: Address
  amount: bigint
  validatorFee: bigint
  listingHash: Address
  requestHash: Address
  deliveryHash: Address
  resolutionHash: Address
  createdAt: bigint
  approvalDeadline: bigint
  deliveryDeadline: bigint
  responseDeadline: bigint
  status: number | bigint
}

export async function readMarketHeader(): Promise<MarketHeaderData> {
  const [marketURI, owner, orderCount] = await Promise.all([
    publicClient.readContract({ ...market, functionName: 'marketURI' }) as Promise<string>,
    publicClient.readContract({ ...market, functionName: 'owner' }) as Promise<Address>,
    publicClient.readContract({ ...market, functionName: 'getOrderCount' }) as Promise<bigint>,
  ])
  return { marketURI, owner, orderCount: Number(orderCount) }
}

export async function readOrder(id: number): Promise<OnChainOrder> {
  const raw = (await publicClient.readContract({
    ...market,
    functionName: 'getOrder',
    args: [BigInt(id)],
  })) as RawOrder

  return {
    id,
    buyer: raw.buyer,
    seller: raw.seller,
    validator: raw.validator,
    amount: raw.amount,
    validatorFee: raw.validatorFee,
    listingHash: raw.listingHash,
    requestHash: raw.requestHash,
    deliveryHash: raw.deliveryHash,
    resolutionHash: raw.resolutionHash,
    createdAt: raw.createdAt,
    approvalDeadline: raw.approvalDeadline,
    deliveryDeadline: raw.deliveryDeadline,
    responseDeadline: raw.responseDeadline,
    status: Number(raw.status),
  }
}

/** Reads orders 1..count in bounded-concurrency chunks. */
export async function readOrders(count: number): Promise<OnChainOrder[]> {
  if (count <= 0) return []

  const ids = Array.from({ length: count }, (_, i) => i + 1)
  const chunkSize = 10
  const orders: OnChainOrder[] = []

  for (let i = 0; i < ids.length; i += chunkSize) {
    const chunk = ids.slice(i, i + chunkSize)
    const results = await Promise.all(chunk.map((id) => readOrder(id)))
    orders.push(...results)
  }

  return orders
}
