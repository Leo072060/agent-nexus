export type Address = `0x${string}`

export interface MarketHeaderData {
  marketURI: string
  owner: Address
  orderCount: number
}

/** Mirrors the on-chain Order struct (MarketStorage.sol). */
export interface OnChainOrder {
  id: number
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
  status: number
}

/** GET /agent-nexus/me */
export interface Me {
  validatorAddress: Address
  marketAddress: Address
  baseURL: string
}

/** Parsed LLM ruling stored by validator-service. */
export interface Decision {
  releaseToSeller: boolean
  summary: string
  reasoning: string
  buyerClaim: string
  sellerDeliveryAssessment: string
  confidence: string
}

/** GET /agent-nexus/disputes item */
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

/** GET /agent-nexus/disputes/{orderId} */
export interface DisputeDetail extends DisputeSummary {
  requestHash: string
  request: string
  deliveryHash: string
  delivery: string
  disputeHash: string
  dispute: string
  decision: Decision | null
}

export type ResolvedOutcome = 'seller' | 'buyer' | null

/** Per-order view model used by the table/drawer. */
export interface OrderRow extends OnChainOrder {
  isMine: boolean
  isDisputed: boolean
  isResolved: boolean
  resolvedOutcome: ResolvedOutcome
  disputeSummary?: DisputeSummary
}
