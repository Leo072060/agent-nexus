# DeepSeek Dispute Validator

Local validator LLM script for `validator-service`. It reads Agent Nexus dispute evidence from stdin, asks DeepSeek for a ruling, validates the response, and prints the decision JSON expected by `validator-service`.

| Item | Value |
|---|---|
| stdin | Validator evidence JSON |
| stdout | Decision JSON |
| stderr | Diagnostics |
| API key | `VALIDATOR_LLM_API_KEY` |
| Endpoint | `https://api.deepseek.com/chat/completions` |
| Model | `deepseek-chat` |

Optional overrides:

| Env | Description |
|---|---|
| `VALIDATOR_LLM_BASE_URL` | DeepSeek-compatible chat completions URL |
| `VALIDATOR_LLM_MODEL` | Chat model name |

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

Configure `validator-service`:

```bash
VALIDATOR_LLM_SCRIPT=/absolute/path/to/validator-llms/deepseek-dispute/run.py
VALIDATOR_LLM_API_KEY=...
```

Run a local protocol test:

```bash
python3 validator-llms/deepseek-dispute/run_test.py
```
