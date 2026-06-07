#!/usr/bin/env bash
#
# Start a local demo Technical Intelligence seller-service instance.
# Reads private settings from .env.local and market settings from cli-frontend/.env.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOCAL_ENV="$ROOT/.env.local"
CLI_FRONTEND_ENV="$ROOT/cli-frontend/.env"

load_env_file() {
  local file="$1"
  while IFS= read -r line || [ -n "$line" ]; do
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    [ -z "$line" ] && continue
    case "$line" in \#*) continue ;; esac
    case "$line" in
      *=*)
        local key="${line%%=*}"
        local value="${line#*=}"
        key="${key#"${key%%[![:space:]]*}"}"
        key="${key%"${key##*[![:space:]]}"}"
        value="${value#"${value%%[![:space:]]*}"}"
        value="${value%"${value##*[![:space:]]}"}"
        value="${value%\"}"
        value="${value#\"}"
        value="${value%\'}"
        value="${value#\'}"
        export "$key=$value"
        ;;
    esac
  done < "$file"
}

if [ -f "$LOCAL_ENV" ]; then
  load_env_file "$LOCAL_ENV"
fi
if [ -f "$CLI_FRONTEND_ENV" ]; then
  load_env_file "$CLI_FRONTEND_ENV"
fi

command -v cast >/dev/null || { echo "Missing cast (foundry)."; exit 1; }

export SELLER_RPC_URL="${SELLER_RPC_URL:-${VITE_RPC_URL:-}}"
export SELLER_MARKET_ADDRESS="${SELLER_MARKET_ADDRESS:-${VITE_MARKET_ADDRESS:-}}"
export SELLER_URI="${SELLER_URI:-http://localhost:8083}"
export SELLER_HTTP_ADDR="${SELLER_HTTP_ADDR:-:8083}"
export SELLER_DB_PATH="${SELLER_DB_PATH:-./seller-service-xapi-sepolia.db}"
export SELLER_LOG_PATH="${SELLER_LOG_PATH:-./seller-service-xapi-sepolia.log}"
export SELLER_SERVICE_ID="${SELLER_SERVICE_ID:-xapi-tech-intel}"
export SELLER_SERVICE_NAME="${SELLER_SERVICE_NAME:-xapi Technical Intelligence}"
export SELLER_SERVICE_DESCRIPTION="${SELLER_SERVICE_DESCRIPTION:-基于固定活动信息源和 DeepSeek 生成技术情报问答。}"
export SELLER_LLM_SCRIPT="${SELLER_LLM_SCRIPT:-$ROOT/seller-llms/xapi-tech-intel/run.py}"
export SELLER_CONTENT_URI="${SELLER_CONTENT_URI:-ipfs://agent-nexus/xapi-tech-intel-v1}"
export SELLER_CONTENT_HASH="${SELLER_CONTENT_HASH:-$(cast keccak "Agent Nexus xapi technical intelligence seller v1")}"
export SELLER_PRICE_WEI="${SELLER_PRICE_WEI:-100000000000000}"
export SELLER_DELIVERY_TIMEOUT="${SELLER_DELIVERY_TIMEOUT:-31536000}"

: "${SELLER_RPC_URL:?SELLER_RPC_URL or VITE_RPC_URL is required}"
: "${SELLER_MARKET_ADDRESS:?SELLER_MARKET_ADDRESS or VITE_MARKET_ADDRESS is required}"
: "${SELLER_PRIVATE_KEY:?SELLER_PRIVATE_KEY is required}"
: "${SELLER_SUPPORTED_VALIDATORS:?SELLER_SUPPORTED_VALIDATORS is required}"
: "${SELLER_LLM_API_KEY:?SELLER_LLM_API_KEY is required as the DeepSeek API key}"

echo "Starting demo Technical Intelligence seller"
echo "  RPC:       $SELLER_RPC_URL"
echo "  Market:    $SELLER_MARKET_ADDRESS"
echo "  URI:       $SELLER_URI"
echo "  HTTP addr: $SELLER_HTTP_ADDR"
echo "  Script:    $SELLER_LLM_SCRIPT"
echo "  LLM:       ${SELLER_LLM_MODEL:-deepseek-chat}"

cd "$ROOT/seller-service"
exec go run ./cmd/seller-service serve
