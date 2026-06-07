import { marketAbi } from '../abi/market'
import { config } from '../config'
import { publicClient } from './client'
import type { Address, MarketHeaderData, OnChainOrder, Seller, Validator } from '../types'

const market = { address: config.marketAddress, abi: marketAbi } as const

type RawOrder = {
  buyer: Address
  seller: Address
  validator: Address
  amount: bigint
  validatorFee: bigint
  validatorBond: bigint
  listingHash: string
  requestHash: string
  deliveryHash: string
  resolutionHash: string
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

export async function readSellers(): Promise<Seller[]> {
  const addresses = (await publicClient.readContract({ ...market, functionName: 'getSellers' })) as Address[]
  return Promise.all(addresses.map(readSeller))
}

export async function readSeller(address: Address): Promise<Seller> {
  const raw = (await publicClient.readContract({ ...market, functionName: 'getSeller', args: [address] })) as readonly unknown[]
  return {
    address,
    registered: Boolean(raw[0]),
    active: Boolean(raw[1]),
    sellerURI: String(raw[2]),
    price: raw[3] as bigint,
    contentURI: String(raw[4]),
    contentHash: String(raw[5]),
    deliveryTimeout: raw[6] as bigint,
  }
}

export async function readValidators(): Promise<Validator[]> {
  const addresses = (await publicClient.readContract({ ...market, functionName: 'getValidators' })) as Address[]
  return Promise.all(addresses.map(readValidator))
}

export async function readValidator(address: Address): Promise<Validator> {
  const raw = (await publicClient.readContract({ ...market, functionName: 'getValidator', args: [address] })) as readonly unknown[]
  return {
    address,
    registered: Boolean(raw[0]),
    active: Boolean(raw[1]),
    validatorURI: String(raw[2]),
    fee: raw[3] as bigint,
    responseTimeout: raw[4] as bigint,
  }
}

export async function readOrder(id: number): Promise<OnChainOrder> {
  const raw = (await publicClient.readContract({ ...market, functionName: 'getOrder', args: [BigInt(id)] })) as RawOrder
  return {
    id,
    buyer: raw.buyer,
    seller: raw.seller,
    validator: raw.validator,
    amount: raw.amount,
    validatorFee: raw.validatorFee,
    validatorBond: raw.validatorBond,
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

export async function readOrders(count: number): Promise<OnChainOrder[]> {
  if (count <= 0) return []
  const ids = Array.from({ length: count }, (_, i) => i + 1)
  const chunks: OnChainOrder[] = []
  for (let i = 0; i < ids.length; i += 10) {
    chunks.push(...(await Promise.all(ids.slice(i, i + 10).map(readOrder))))
  }
  return chunks
}
