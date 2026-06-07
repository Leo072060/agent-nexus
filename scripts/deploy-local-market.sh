#!/usr/bin/env bash
#
# Deploy a clean local Agent Nexus Market to Anvil.
# This starts/reuses Anvil, deploys only Market, and writes cli-frontend/.env.
# It does not register sellers/validators or create demo orders.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RPC="http://127.0.0.1:8545"
ARTIFACT="$ROOT/out/Market.sol/Market.json"
ANVIL_LOG="$ROOT/scripts/anvil.log"

K0=0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 # deployer
K1=0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d # seller
K3=0x7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6 # validator
K5=0x8b3a350cf5c34c9194ca85829a2df0ec3153be0318b5e2d3348e872092edffba # buyer

echo "==> Agent Nexus clean local Market deploy"

command -v anvil >/dev/null || { echo "Missing anvil (foundry)."; exit 1; }
command -v cast >/dev/null || { echo "Missing cast (foundry)."; exit 1; }
command -v jq >/dev/null || { echo "Missing jq."; exit 1; }
[ -f "$ARTIFACT" ] || { echo "Missing artifact $ARTIFACT. Run: forge build"; exit 1; }

echo "[1/3] Preparing Anvil..."
if cast block-number --rpc-url "$RPC" >/dev/null 2>&1; then
  echo "    Reusing existing Anvil at $RPC"
else
  nohup anvil --silent --host 127.0.0.1 --port 8545 > "$ANVIL_LOG" 2>&1 < /dev/null &
  ANVIL_PID=$!
  disown "$ANVIL_PID" 2>/dev/null || true
  ready=0
  for _ in $(seq 1 50); do
    cast block-number --rpc-url "$RPC" >/dev/null 2>&1 && { ready=1; break; }
    sleep 0.2 2>/dev/null || true
  done
  [ "$ready" = 1 ] || { echo "Anvil failed to start. See $ANVIL_LOG"; exit 1; }
  echo "    Anvil started (log: $ANVIL_LOG)"
fi

echo "[2/3] Deploying Market..."
BYTECODE=$(jq -r '.bytecode.object' "$ARTIFACT")
CARGS=$(cast abi-encode "constructor(string)" "ipfs://agent-nexus-demo-market")
INIT="${BYTECODE}${CARGS#0x}"
MARKET=$(cast send --rpc-url "$RPC" --private-key "$K0" --create "$INIT" --json | jq -r .contractAddress)
[ -n "$MARKET" ] && [ "$MARKET" != "null" ] || { echo "Market deployment did not return a contract address."; exit 1; }
echo "    MARKET_ADDRESS=$MARKET"

cast block-number --rpc-url "$RPC" >/dev/null 2>&1 || {
  echo "Anvil is not responding after deployment. See $ANVIL_LOG"
  exit 1
}

echo "[3/3] Writing cli-frontend/.env..."
mkdir -p "$ROOT/cli-frontend"
cat > "$ROOT/cli-frontend/.env" <<ENV
VITE_RPC_URL=$RPC
VITE_MARKET_ADDRESS=$MARKET
VITE_POLL_MS=8000
ENV

DEPLOYER=$(cast wallet address --private-key "$K0")
SELLER=$(cast wallet address --private-key "$K1")
VALIDATOR=$(cast wallet address --private-key "$K3")
BUYER=$(cast wallet address --private-key "$K5")

cat <<DONE

==================== Clean local Market ready ====================
RPC_URL=$RPC
MARKET_ADDRESS=$MARKET

Default Anvil demo wallets:
  deployer  $DEPLOYER
    private $K0
  seller    $SELLER
    private $K1
  validator $VALIDATOR
    private $K3
  buyer     $BUYER
    private $K5

cli-frontend/.env has been written.

Sanity checks:
  cast call $MARKET "getOrderCount()(uint256)" --rpc-url $RPC
  cast call $MARKET "getSellers()(address[])" --rpc-url $RPC
  cast call $MARKET "getValidators()(address[])" --rpc-url $RPC

Stop local chain:
  pkill -f 'anvil --silent'
==================================================================
DONE
