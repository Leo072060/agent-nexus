# Seller Service

## Database Design

SQLite database path is configured by `SELLER_DB_PATH`.
If omitted, the default path is `./seller-service.db`.

### `orders`

| Column | Type | Constraints | Description |
|---|---|---|---|
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | Local row id |
| `chain_order_id` | TEXT | NOT NULL UNIQUE | On-chain order id |
| `buyer_address` | TEXT | NOT NULL | Buyer wallet address |
| `seller_address` | TEXT | NOT NULL | Seller wallet address |
| `validator_address` | TEXT | NOT NULL | Validator wallet address |
| `request_hash` | TEXT | NOT NULL | Keccak256 hash of buyer request |
| `request_body` | BLOB | NOT NULL | Raw buyer request body |
| `confirm_seller_tx_hash` | TEXT | NOT NULL DEFAULT '' | Seller confirmation transaction hash |
| `delivery_hash` | TEXT | NOT NULL DEFAULT '' | Keccak256 hash of delivery body |
| `delivery_body` | BLOB | nullable | Raw answer text returned by local LLM script |
| `commit_delivery_tx_hash` | TEXT | NOT NULL DEFAULT '' | Delivery commit transaction hash |
| `evidence_hash` | TEXT | NOT NULL DEFAULT '' | Keccak256 hash of seller evidence |
| `evidence_body` | BLOB | nullable | Seller evidence text for validator |
| `evidence_sent_at` | TEXT | NOT NULL DEFAULT '' | UTC RFC3339 evidence send time |
| `evidence_post_status` | INTEGER | NOT NULL DEFAULT 0 | Validator evidence POST status |
| `evidence_post_response` | TEXT | NOT NULL DEFAULT '' | Validator evidence POST response |
| `status` | TEXT | NOT NULL | Local order status |
| `created_at` | TEXT | NOT NULL | UTC RFC3339 creation time |
| `updated_at` | TEXT | NOT NULL | UTC RFC3339 update time |

## HTTP API

| Method | Path | Purpose |
|---|---|---|
| GET | `/health` | Return service health |
| GET | `/agent-nexus/services` | Return seller service metadata |
| POST | `/agent-nexus/request` | Verify buyer request, store it, and call `confirmAsSeller` |
| POST | `/agent-nexus/delivery` | Verify buyer delivery request and return stored answer text |

## Service Metadata

| Config | Required | Description |
|---|---|---|
| `SELLER_SERVICE_ID` | yes | Stable service id |
| `SELLER_SERVICE_NAME` | yes | Human-readable service name |
| `SELLER_SERVICE_DESCRIPTION` | yes | Short service description |
| `SELLER_LOG_PATH` | no | Log file path, default `./seller-service.log` |

## On-chain Seller Config

seller-service checks this config before starting HTTP and watcher. If the on-chain seller is missing or differs, it asks for confirmation before sending register/update transactions.

| Config | Required | Description |
|---|---|---|
| `SELLER_URI` | yes | On-chain seller URI |
| `SELLER_PRICE_WEI` | yes | Product price in wei |
| `SELLER_CONTENT_URI` | yes | Product content URI |
| `SELLER_CONTENT_HASH` | yes | Non-zero bytes32 content hash |
| `SELLER_DELIVERY_TIMEOUT` | yes | Delivery timeout in seconds |
| `SELLER_SUPPORTED_VALIDATORS` | yes | Comma-separated validator addresses |

## Chain Watcher

| Chain Status | Action |
|---|---|
| `PendingSeller` | If local request exists and hash matches chain, call `confirmAsSeller` |
| `Created` | Run local LLM script, store answer/evidence, and commit answer hash on-chain |
| `Disputed` | POST request, answer, and seller evidence to the order validator URI |
| Other | Ignore |

## Local LLM Script Interface

| Config | Required | Description |
|---|---|---|
| `SELLER_LLM_SCRIPT` | yes | Executable script path |
| `SELLER_LLM_API_KEY` | yes | Passed to script env |
| `SELLER_LLM_TIMEOUT` | no | Script timeout, default `60s` |

stdin:

```text
buyer request text
```

stdout:

```json
{
  "answer": "model answer for buyer",
  "evidence": "seller evidence for validator"
}
```

Only `answer` is returned to the buyer. `evidence` is sent to the validator if the order becomes disputed.
stderr is for diagnostics. A non-zero exit code means failure.
