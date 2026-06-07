# TAS Diagram

Uses DeepSeek to organize built-in TAS diagram reference data.

| Item | Value |
|---|---|
| stdin | Buyer request text |
| stdout | JSON with `answer` and `evidence` |
| stderr | Diagnostics |
| API key | `SELLER_LLM_API_KEY` |
| Endpoint | `https://api.deepseek.com/chat/completions` |
| Model | `deepseek-v4-flash` |

DeepSeek first decides what supporting data it needs. The script then supplies
only the requested TAS data blocks and prints JSON for seller-service.

| Info key | Content |
|---|---|
| `tas.volcanic.en` | Volcanic TAS with English labels |
| `tas.volcanic.zh` | Volcanic TAS with Chinese labels |
| `tas.plutonic.en` | Plutonic TAS with English labels |
| `tas.plutonic.zh` | Plutonic TAS with Chinese labels |
| `tas.all` | All TAS blocks |

DeepSeek must return protocol JSON internally:

```json
{"status":"need_info","requests":["tas.volcanic.en"]}
```

or:

```json
{"status":"final","answer":"plain text answer","evidence":"plain text seller evidence"}
```

Script stdout:

```json
{"answer":"plain text answer","evidence":"plain text seller evidence"}
```

Run:

```bash
echo "Please provide volcanic TAS data in English" | SELLER_LLM_API_KEY=... ./run.py
echo "请给我深成岩 TAS 中文标签" | SELLER_LLM_API_KEY=... ./run.py
```
