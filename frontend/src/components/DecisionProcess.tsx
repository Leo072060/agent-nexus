import { useEffect, useState } from 'react'
import { fetchDisputeDetail } from '../api/validator'
import { sameHex } from '../lib/format'
import type { DisputeDetail, OrderRow } from '../types'

function Evidence({ label, hash, body }: { label: string; hash: string; body: string }) {
  return (
    <details className="evidence">
      <summary>{label}</summary>
      <div className="evidence-hash mono">{hash || '—'}</div>
      <pre className="evidence-body">{body || '(空)'}</pre>
    </details>
  )
}

export function DecisionProcess({ row }: { row: OrderRow }) {
  const [detail, setDetail] = useState<DisputeDetail | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    setDetail(null)
    fetchDisputeDetail(row.id)
      .then((d) => {
        if (!cancelled) setDetail(d)
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [row.id])

  if (loading) return <div className="dp loading-inline">加载判决流程…</div>

  if (error) {
    return (
      <div className="dp">
        <div className="dp-title">
          我的判决流程 <span className="private-tag">Private</span>
        </div>
        <div className="error-inline">未找到本地裁决记录：{error}</div>
        <p className="muted small">
          该订单由我裁决，但 validator-service 中暂无证据/裁决记录（可能证据尚未提交，或数据库与此服务不同）。
        </p>
      </div>
    )
  }

  if (!detail) return null

  const hashOk = sameHex(detail.resolutionHash, row.resolutionHash)

  return (
    <div className="dp">
      <div className="dp-title">
        我的判决流程 <span className="private-tag">Private</span>
      </div>

      <section className="dp-section">
        <h4>证据原文</h4>
        <Evidence label="买家请求 request" hash={detail.requestHash} body={detail.request} />
        <Evidence label="卖家交付 delivery" hash={detail.deliveryHash} body={detail.delivery} />
        <Evidence label="争议理由 dispute" hash={detail.disputeHash} body={detail.dispute} />
      </section>

      <section className="dp-section">
        <h4>LLM 裁决</h4>
        {detail.decision ? (
          <div className="decision-card">
            <div className="decision-outcome">
              <span className={detail.decision.releaseToSeller ? 'outcome-seller' : 'outcome-buyer'}>
                {detail.decision.releaseToSeller ? '放款给卖家' : '退款给买家'}
              </span>
              <span className="confidence">置信度 {detail.decision.confidence}</span>
            </div>
            <p>
              <strong>结论：</strong>
              {detail.decision.summary}
            </p>
            <p>
              <strong>理由：</strong>
              {detail.decision.reasoning}
            </p>
            <p>
              <strong>买家主张：</strong>
              {detail.decision.buyerClaim}
            </p>
            <p>
              <strong>交付评估：</strong>
              {detail.decision.sellerDeliveryAssessment}
            </p>
          </div>
        ) : (
          <div className="muted">无 LLM 裁决记录</div>
        )}
      </section>

      <section className="dp-section">
        <h4>状态与凭证</h4>
        <div className="timeline">
          <span className="tl-step done">证据已接收</span>
          <span
            className={`tl-step ${
              detail.status === 'resolved' ? 'done' : detail.status === 'resolution_failed' ? 'failed' : 'pending'
            }`}
          >
            {detail.status === 'resolution_failed' ? '裁决失败' : detail.status === 'resolved' ? '已裁决' : '待裁决'}
          </span>
        </div>
        <dl className="kv">
          <dt>resolutionHash</dt>
          <dd className="mono">
            {detail.resolutionHash || '—'}
            {detail.resolutionHash && (
              <span className={hashOk ? 'ok' : 'warn-text'}> {hashOk ? '✓ 与链上一致' : '⚠ 与链上不一致'}</span>
            )}
          </dd>
          <dt>resolveTxHash</dt>
          <dd className="mono">{detail.resolveTxHash || '—'}</dd>
          <dt>更新时间</dt>
          <dd>{detail.updatedAt}</dd>
        </dl>
      </section>
    </div>
  )
}
