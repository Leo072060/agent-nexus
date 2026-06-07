import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { config } from '../config'
import { fetchDisputeSummaries } from '../api/validator'
import { readMarketHeader, readOrders, readSellers, readValidators } from '../chain/market'
import type { Address, DisputeSummary, MarketHeaderData, OnChainOrder, OrderRow, Seller, Validator } from '../types'

interface MarketState {
  loading: boolean
  error: string | null
  header: MarketHeaderData | null
  sellers: Seller[]
  validators: Validator[]
  orders: OnChainOrder[]
  disputes: Map<string, DisputeSummary>
  validatorErrors: string[]
  lastUpdated: number | null
}

const INITIAL: MarketState = {
  loading: true,
  error: null,
  header: null,
  sellers: [],
  validators: [],
  orders: [],
  disputes: new Map(),
  validatorErrors: [],
  lastUpdated: null,
}

export function useMarketData() {
  const [state, setState] = useState<MarketState>(INITIAL)
  const inFlight = useRef(false)

  const load = useCallback(async () => {
    if (inFlight.current) return
    inFlight.current = true
    try {
      const [header, sellers, validators] = await Promise.all([readMarketHeader(), readSellers(), readValidators()])
      const orders = await readOrders(header.orderCount)
      const validatorByAddress = new Map(validators.map((validator) => [validator.address.toLowerCase(), validator]))
      const validatorURIs = Array.from(
        new Set(
          orders
            .map((order) => validatorByAddress.get(order.validator.toLowerCase())?.validatorURI)
            .filter((uri): uri is string => Boolean(uri && /^https?:\/\//.test(uri))),
        ),
      )

      const disputeResults = await Promise.allSettled(validatorURIs.map((uri) => fetchDisputeSummaries(uri)))
      const disputes = new Map<string, DisputeSummary>()
      const validatorErrors: string[] = []
      disputeResults.forEach((result, index) => {
        if (result.status === 'fulfilled') {
          for (const dispute of result.value) disputes.set(dispute.orderId, dispute)
        } else {
          validatorErrors.push(`${validatorURIs[index]}: ${result.reason instanceof Error ? result.reason.message : String(result.reason)}`)
        }
      })

      setState({ loading: false, error: null, header, sellers, validators, orders, disputes, validatorErrors, lastUpdated: Date.now() })
    } catch (e) {
      setState((prev) => ({ ...prev, loading: false, error: e instanceof Error ? e.message : String(e), lastUpdated: Date.now() }))
    } finally {
      inFlight.current = false
    }
  }, [])

  useEffect(() => {
    void load()
    if (config.pollMs > 0) {
      const timer = window.setInterval(() => void load(), config.pollMs)
      return () => window.clearInterval(timer)
    }
    return undefined
  }, [load])

  const rows: OrderRow[] = useMemo(() => {
    const sellerByAddress = new Map(state.sellers.map((seller) => [seller.address.toLowerCase(), seller]))
    const validatorByAddress = new Map(state.validators.map((validator) => [validator.address.toLowerCase(), validator]))
    return state.orders.map((order) => ({
      ...order,
      sellerInfo: sellerByAddress.get(order.seller.toLowerCase()),
      validatorInfo: validatorByAddress.get(order.validator.toLowerCase()),
      disputeSummary: state.disputes.get(String(order.id)),
    }))
  }, [state.sellers, state.validators, state.orders, state.disputes])

  const activeSellers = useMemo(() => state.sellers.filter((seller) => seller.active).length, [state.sellers])
  const activeValidators = useMemo(() => state.validators.filter((validator) => validator.active).length, [state.validators])

  return {
    ...state,
    rows,
    activeSellers,
    activeValidators,
    refresh: load,
  }
}

export function getValidatorURI(row: OrderRow): string {
  return row.validatorInfo?.validatorURI ?? ''
}

export function addressIsParticipant(row: OrderRow, address?: Address | null): boolean {
  if (!address) return false
  const value = address.toLowerCase()
  return row.buyer.toLowerCase() === value || row.seller.toLowerCase() === value || row.validator.toLowerCase() === value
}
