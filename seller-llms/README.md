# Seller LLM Scripts

Each script is executed once per delivery.

| Stream | Meaning |
|---|---|
| stdin | Buyer request text |
| stdout | JSON with final answer and seller evidence |
| stderr | Diagnostics |
| exit code 0 | Success |
| non-zero exit | Failure |

Scripts read the model key from `SELLER_LLM_API_KEY`.
Do not write API keys to stdout or stderr.

The TAS diagram script lets DeepSeek request built-in TAS data blocks and returns
JSON containing DeepSeek's final answer and seller evidence.

Example layout:

```text
seller-llms/
  tas-diagram/
    run.py
  contract-review/
    run.py
  translation/
    run.py
  code-audit/
    run.py
```

Scripts should be executable and include a shebang, for example `#!/usr/bin/env python3`.

Example:

```bash
SELLER_LLM_SCRIPT="/Users/twh/Projects/Agent Nexus/seller-llms/tas-diagram/run.py"
```
