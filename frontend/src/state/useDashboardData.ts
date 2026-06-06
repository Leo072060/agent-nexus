import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { config } from '../config'
import { readMarketHeader, readOrders } from '../chain/market'
import { fetchDisputes, fetchMe } from '../api/validator'
import { sameAddress } from '../lib/format'
import { isDisputedStatus, isResolvedStatus, resolvedOutcome } from '../lib/status'
import type { DisputeSummary, MarketHeaderData, Me, OnChainOrder, OrderRow } from '../types'

interface DashboardState {
  loading: boolean
  error: string | null
  header: MarketHeaderData | null
  orders: OnChainOrder[]
  me: Me | null
  meError: string | null
  disputes: Map<string, DisputeSummary>
  lastUpdated: number | null
}

const INITIAL: DashboardState = {
  loading: true,
  error: null,
  header: null,
  orders: [],
  me: null,
  meError: null,
  disputes: new Map(),
  lastUpdated: null,
}

export function useDashboardData() {
  const [state, setState] = useState<DashboardState>(INITIAL)
  const [viewAs, setViewAs] = useState<string>(config.defaultViewAs)
  const inFlight = useRef(false)

  const load = useCallback(async () => {
    if (inFlight.current) return
    inFlight.current = true
    try {
      // Public on-chain reads (required).
      const header = await readMarketHeader()
      const orders = await readOrders(header.orderCount)

      // Private validator-service reads (optional — service may be offline).
      let me: Me | null = null
      let meError: string | null = null
      let disputeList: DisputeSummary[] = []
      try {
        const [meRes, disputesRes] = await Promise.all([fetchMe(), fetchDisputes()])
        me = meRes
        disputeList = disputesRes
      } catch (e) {
        meError = e instanceof Error ? e.message : String(e)
      }

      const disputes = new Map(disputeList.map((d) => [d.orderId, d]))
      setState({ loading: false, error: null, header, orders, me, meError, disputes, lastUpdated: Date.now() })
    } catch (e) {
      setState((prev) => ({
        ...prev,
        loading: false,
        error: e instanceof Error ? e.message : String(e),
        lastUpdated: Date.now(),
      }))
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

  // "我" = manual override if present, otherwise the validator-service's own address.
  const effectiveMe = (viewAs.trim() || state.me?.validatorAddress || '').trim()

  const rows: OrderRow[] = useMemo(
    () =>
      state.orders.map((order) => ({
        ...order,
        isMine: sameAddress(order.validator, effectiveMe),
        isDisputed: isDisputedStatus(order.status),
        isResolved: isResolvedStatus(order.status),
        resolvedOutcome: resolvedOutcome(order.status),
        disputeSummary: state.disputes.get(String(order.id)),
      })),
    [state.orders, state.disputes, effectiveMe],
  )

  const marketMismatch = state.me != null && !sameAddress(state.me.marketAddress, config.marketAddress)

  return {
    loading: state.loading,
    error: state.error,
    header: state.header,
    me: state.me,
    meError: state.meError,
    lastUpdated: state.lastUpdated,
    rows,
    viewAs,
    setViewAs,
    effectiveMe,
    marketMismatch,
    refresh: load,
  }
}
