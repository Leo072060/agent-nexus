# Agent Nexus Local Demo Environment

This file records the current local demo chain, service endpoints, role wallets, and CLI setup hints.
Do not write real API keys or private production keys here.

## Chain

| Item | Value |
| --- | --- |
| RPC URL | `http://127.0.0.1:8545` |
| Market | `0x5fbdb2315678afecb367f032d93f642f64180aa3` |
| Market URI | `ipfs://agent-nexus-demo-market` |
| Frontend | `http://127.0.0.1:5173` |

`cli-frontend/.env`:

```bash
VITE_RPC_URL=http://127.0.0.1:8545
VITE_MARKET_ADDRESS=0x5fbdb2315678afecb367f032d93f642f64180aa3
VITE_POLL_MS=8000
```

## Validator

| Item | Value |
| --- | --- |
| Address | `0x90F79bf6EB2c4f870365E785982E1f101E93b906` |
| URI | `http://localhost:8082` |
| Health | `http://localhost:8082/health` |
| Local DB | `validator-service/validator-service-local.db` |
| LLM script | `/Users/twh/Projects/Agent Nexus/validator-llms/deepseek-dispute/run.py` |

Runtime env:

```bash
VALIDATOR_RPC_URL=http://127.0.0.1:8545
VALIDATOR_MARKET_ADDRESS=0x5fbdb2315678afecb367f032d93f642f64180aa3
VALIDATOR_PRIVATE_KEY=<validator private key>
VALIDATOR_BASE_URL=http://localhost:8082
VALIDATOR_HTTP_ADDR=:8082
VALIDATOR_DB_PATH=./validator-service-local.db
VALIDATOR_LLM_SCRIPT=/Users/twh/Projects/Agent Nexus/validator-llms/deepseek-dispute/run.py
VALIDATOR_LLM_API_KEY=<DeepSeek key>
```

## Sellers

| Service | Address | URI | Port | Description |
| --- | --- | --- | --- | --- |
| `xapi-tech-intel` | `0x70997970C51812dc3A010C7d01b50e0d17dc79C8` | `http://localhost:8083` | `8083` | Fixed event source + DeepSeek answer for PKU Blockchain Association activity. |
| `tas-diagram` | `0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC` | `http://localhost:8081` | `8081` | TAS diagram guidance with built-in TAS reference data and DeepSeek. |

Both sellers support validator:

```text
0x90F79bf6EB2c4f870365E785982E1f101E93b906
```

### xapi-tech-intel seller env

```bash
SELLER_RPC_URL=http://127.0.0.1:8545
SELLER_MARKET_ADDRESS=0x5fbdb2315678afecb367f032d93f642f64180aa3
SELLER_PRIVATE_KEY=<xapi-tech seller private key>
SELLER_URI=http://localhost:8083
SELLER_HTTP_ADDR=:8083
SELLER_DB_PATH=./seller-service-xapi-sepolia.db
SELLER_LOG_PATH=./seller-service-xapi-sepolia.log
SELLER_SERVICE_ID=xapi-tech-intel
SELLER_SERVICE_NAME=xapi Technical Intelligence
SELLER_SERVICE_DESCRIPTION=基于固定活动信息源和 DeepSeek 生成技术情报问答。
SELLER_LLM_SCRIPT=/Users/twh/Projects/Agent Nexus/seller-llms/xapi-tech-intel/run.py
SELLER_LLM_API_KEY=<DeepSeek key>
SELLER_CONTENT_URI=ipfs://agent-nexus/xapi-tech-intel-v1
SELLER_CONTENT_HASH=0x46b87875e95439e7f46007c9e96d7a3231c00bd8f1c8dac4f8fb94b4615fd555
SELLER_PRICE_WEI=100000000000000
SELLER_DELIVERY_TIMEOUT=31536000
SELLER_SUPPORTED_VALIDATORS=0x90F79bf6EB2c4f870365E785982E1f101E93b906
```

### tas-diagram seller env

```bash
SELLER_RPC_URL=http://127.0.0.1:8545
SELLER_MARKET_ADDRESS=0x5fbdb2315678afecb367f032d93f642f64180aa3
SELLER_PRIVATE_KEY=<tas seller private key>
SELLER_URI=http://localhost:8081
SELLER_HTTP_ADDR=:8081
SELLER_DB_PATH=./seller-service-tas-local.db
SELLER_LOG_PATH=./seller-service-tas-local.log
SELLER_SERVICE_ID=tas-diagram
SELLER_SERVICE_NAME=TAS Diagram Agent
SELLER_SERVICE_DESCRIPTION=Generates TAS diagram guidance with DeepSeek and built-in TAS reference data.
SELLER_LLM_SCRIPT=/Users/twh/Projects/Agent Nexus/seller-llms/tas-diagram/run.py
SELLER_LLM_API_KEY=<DeepSeek key>
SELLER_CONTENT_URI=ipfs://agent-nexus/tas-diagram-v1
SELLER_CONTENT_HASH=0x4673d32817423a457537fe96aa2e72176eee848667d97f5d8ac233d70f5d204b
SELLER_PRICE_WEI=100000000000000
SELLER_DELIVERY_TIMEOUT=31536000
SELLER_SUPPORTED_VALIDATORS=0x90F79bf6EB2c4f870365E785982E1f101E93b906
```

## Default Local Wallets

These are public Anvil demo keys only. Do not use them outside local testing.

| Role | Address | Env |
| --- | --- | --- |
| Deployer | `0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266` | `DEPLOYER_PRIVATE_KEY` |
| xapi-tech seller | `0x70997970C51812dc3A010C7d01b50e0d17dc79C8` | `SELLER_PRIVATE_KEY` |
| tas seller | `0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC` | `SELLER_PRIVATE_KEY` |
| Validator | `0x90F79bf6EB2c4f870365E785982E1f101E93b906` | `VALIDATOR_PRIVATE_KEY` |
| Buyer | `0x9965507D1a55bcC2695C58ba16FB37d819B0A4dc` | `BUYER_PRIVATE_KEY` |

## CLI Setup Hints

Use a local buyer database:

```bash
export AGENT_NEXUS_DB=/Users/twh/Projects/Agent Nexus/display/buyer-local.db
export BUYER_PRIVATE_KEY=<buyer private key>
```

Add and activate the local market:

```bash
./cli/cmd/agent-nexus/agent-nexus market add \
  --name local-demo \
  --rpc-url http://127.0.0.1:8545 \
  --market-address 0x5fbdb2315678afecb367f032d93f642f64180aa3

./cli/cmd/agent-nexus/agent-nexus market use --name local-demo
```

Useful service checks:

```bash
curl http://localhost:8082/health
curl http://localhost:8083/agent-nexus/services
curl http://localhost:8081/agent-nexus/services
```

## Current Chain Sanity Checks

```bash
cast call 0x5fbdb2315678afecb367f032d93f642f64180aa3 "getValidators()(address[])" --rpc-url http://127.0.0.1:8545
cast call 0x5fbdb2315678afecb367f032d93f642f64180aa3 "getSellers()(address[])" --rpc-url http://127.0.0.1:8545
cast call 0x5fbdb2315678afecb367f032d93f642f64180aa3 "getOrderCount()(uint256)" --rpc-url http://127.0.0.1:8545
```
