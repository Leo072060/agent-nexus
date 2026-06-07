import { useState } from 'react'
import { fetchDisputeDetail, fetchNonce } from '../api/validator'
import { signMessage } from '../api/wallet'
import { formatAmount, formatTimestamp, sameAddress, shortAddress } from '../lib/format'
import { isDisputeRelated } from '../lib/status'
import { StatusChip } from './StatusChip'
import type { Address, DisputeDetail, OrderRow } from '../types'

export function OrderDetail({
  row,
  wallet,
  onClose,
}: {
  row: OrderRow
  wallet: Address | null
  onClose: () => void
}) {
  const [detail, setDetail] = useState<DisputeDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const validatorURI = row.validatorInfo?.validatorURI ?? ''
  const validatorHost = hostFromURI(validatorURI)
  const isParticipant =
    wallet != null && (sameAddress(wallet, row.buyer) || sameAddress(wallet, row.seller) || sameAddress(wallet, row.validator))
  const canRequestDetail = isDisputeRelated(row.status) && validatorURI !== '' && wallet != null && isParticipant

  async function loadPrivateDetail() {
    if (!wallet || !validatorURI) return
    setLoading(true)
    setError(null)
    try {
      const nonce = await fetchNonce(validatorURI, wallet, row.id)
      const signature = await signMessage(wallet, nonce.message)
      setDetail(await fetchDisputeDetail(validatorURI, row.id, wallet, nonce.nonce, signature))
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }

  return (
    <aside className="detail">
      <div className="detail-head">
        <div>
          <span className="label">订单</span>
          <h2>#{row.id}</h2>
        </div>
        <button type="button" className="ghost" onClick={onClose}>
          关闭
        </button>
      </div>

      <dl className="kv">
        <dt>状态</dt>
        <dd>
          <StatusChip status={row.status} />
        </dd>
        <dt>买家</dt>
        <dd className="mono">{row.buyer}</dd>
        <dt>卖家</dt>
        <dd className="mono">{row.seller}</dd>
        <dt>验证者</dt>
        <dd className="mono">{row.validator}</dd>
        <dt>商品金额</dt>
        <dd>{formatAmount(row.amount)}</dd>
        <dt>验证费</dt>
        <dd>{formatAmount(row.validatorFee)}</dd>
        <dt>验证者质押</dt>
        <dd>{formatAmount(row.validatorBond)}</dd>
        <dt>请求哈希</dt>
        <dd className="mono">{row.requestHash}</dd>
        <dt>交付哈希</dt>
        <dd className="mono">{row.deliveryHash}</dd>
        <dt>裁决哈希</dt>
        <dd className="mono">{row.resolutionHash}</dd>
        <dt>确认截止</dt>
        <dd>{formatTimestamp(row.approvalDeadline)}</dd>
        <dt>交付截止</dt>
        <dd>{formatTimestamp(row.deliveryDeadline)}</dd>
        <dt>响应截止</dt>
        <dd>{formatTimestamp(row.responseDeadline)}</dd>
      </dl>

      {row.disputeSummary && (
        <section className="summary-box">
          <div className="section-head">
            <h3>争议摘要</h3>
            <span>{row.disputeSummary.status}</span>
          </div>
          <p>
            裁决方向：
            <strong>{row.disputeSummary.releaseToSeller ? '卖家' : '买家'}</strong>
          </p>
          <p className="mono">{shortAddress(row.disputeSummary.resolveTxHash || '0x')}</p>
        </section>
      )}

      {isDisputeRelated(row.status) && (
        <section className="private-box">
          <div className="section-head">
            <h3>诉讼详情</h3>
            {validatorHost && <span>{validatorHost}</span>}
          </div>
          {!wallet && <p className="muted">连接钱包后可请求详情。</p>}
          {wallet && !isParticipant && <p className="muted">当前钱包不是该订单参与方。</p>}
          {wallet && isParticipant && !validatorURI && <p className="muted">验证者 URI 不可用。</p>}
          {canRequestDetail && (
            <button type="button" onClick={() => void loadPrivateDetail()} disabled={loading}>
              {loading ? '签名中' : '签名查看'}
            </button>
          )}
          {error && <p className="inline-error">{error}</p>}
          {detail && (
            <div className="private-detail">
              <h4>Request</h4>
              <pre>{detail.request}</pre>
              <h4>Delivery</h4>
              <pre>{detail.delivery}</pre>
              <h4>Dispute</h4>
              <pre>{detail.dispute}</pre>
              {detail.decision && (
                <>
                  <h4>Decision</h4>
                  <dl className="kv compact">
                    <dt>结果</dt>
                    <dd>{detail.decision.releaseToSeller ? '放款给卖家' : '退款给买家'}</dd>
                    <dt>摘要</dt>
                    <dd>{detail.decision.summary}</dd>
                    <dt>理由</dt>
                    <dd>{detail.decision.reasoning}</dd>
                    <dt>置信度</dt>
                    <dd>{detail.decision.confidence}</dd>
                  </dl>
                </>
              )}
            </div>
          )}
        </section>
      )}
    </aside>
  )
}

function hostFromURI(value: string): string {
  try {
    return value ? new URL(value).host : ''
  } catch {
    return ''
  }
}
