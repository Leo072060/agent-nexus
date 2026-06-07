# Agent Nexus Validator Service

Validator service receives buyer dispute evidence, verifies it against the market contract, asks a local LLM script for a ruling, stores the decision, and commits the resolution hash on-chain.

## Configuration

| Config | Required | Description |
|---|---|---|
| `VALIDATOR_RPC_URL` | yes | Ethereum JSON-RPC URL |
| `VALIDATOR_MARKET_ADDRESS` | yes | Market contract address |
| `VALIDATOR_PRIVATE_KEY` | yes | Validator wallet private key |
| `VALIDATOR_BASE_URL` | yes | Public base URL for this validator service |
| `VALIDATOR_LLM_SCRIPT` | yes | Executable local decision script |
| `VALIDATOR_LLM_API_KEY` | yes | Passed to the script as `VALIDATOR_LLM_API_KEY` |
| `VALIDATOR_LLM_TIMEOUT` | no | Script timeout, default `60s` |
| `VALIDATOR_DB_PATH` | no | SQLite path, default `./validator-service.db` |
| `VALIDATOR_HTTP_ADDR` | no | HTTP bind address, default `:8082` |

Example DeepSeek script:

```bash
VALIDATOR_LLM_SCRIPT=/Users/twh/Projects/Agent\ Nexus/validator-llms/deepseek-dispute/run.py
VALIDATOR_LLM_API_KEY=...
```

## Local LLM Script Interface

The Go service does not call a model provider directly. It writes complete dispute evidence to the configured script over stdin and expects a decision JSON on stdout.

stdin:

```json
{
  "marketAddress": "0x...",
  "orderId": "12",
  "buyerAddress": "0x...",
  "sellerAddress": "0x...",
  "validatorAddress": "0x...",
  "requestHash": "0x...",
  "deliveryHash": "0x...",
  "request": "buyer request text",
  "delivery": "seller delivery text",
  "dispute": "buyer dispute text"
}
```

stdout:

```json
{
  "releaseToSeller": true,
  "summary": "short ruling summary",
  "reasoning": "decision reasoning",
  "buyerClaim": "claim summary",
  "sellerDeliveryAssessment": "assessment summary",
  "confidence": "low|medium|high"
}
```

`summary` and `reasoning` must be non-empty. Empty `confidence` defaults to `medium`. stderr is for diagnostics. A non-zero exit code or timeout means the dispute decision failed.
