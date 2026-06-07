export type Address = `0x${string}`

export interface MarketHeaderData {
  marketURI: string
  owner: Address
  orderCount: number
}

export interface Seller {
  address: Address
  registered: boolean
  active: boolean
  sellerURI: string
  price: bigint
  contentURI: string
  contentHash: string
  deliveryTimeout: bigint
}

export interface Validator {
  address: Address
  registered: boolean
  active: boolean
  validatorURI: string
  fee: bigint
  responseTimeout: bigint
}

export interface OnChainOrder {
  id: number
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
  status: number
}

export interface Decision {
  releaseToSeller: boolean
  summary: string
  reasoning: string
  buyerClaim: string
  sellerDeliveryAssessment: string
  confidence: string
}

export interface DisputeSummary {
  orderId: string
  buyerAddress: string
  sellerAddress: string
  validatorAddress: string
  status: string
  releaseToSeller: boolean
  resolutionHash: string
  resolveTxHash: string
  createdAt: string
  updatedAt: string
}

export interface DisputeDetail extends DisputeSummary {
  requestHash: string
  request: string
  deliveryHash: string
  delivery: string
  disputeHash: string
  dispute: string
  decision: Decision | null
}

export interface OrderRow extends OnChainOrder {
  sellerInfo?: Seller
  validatorInfo?: Validator
  disputeSummary?: DisputeSummary
}
