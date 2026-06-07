#!/usr/bin/env bash
#
# Start validator-service for local development with a real validator LLM script.
# Reads private LLM settings from .env.local and optional Market settings from cli-frontend/.env.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOCAL_ENV="$ROOT/.env.local"
CLI_FRONTEND_ENV="$ROOT/cli-frontend/.env"

if [ ! -f "$LOCAL_ENV" ]; then
  echo "Missing $LOCAL_ENV"
  echo "Create it with: cp .env.example .env.local"
  echo "Then fill VALIDATOR_LLM_SCRIPT and VALIDATOR_LLM_API_KEY."
  exit 1
fi

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

load_env_file "$LOCAL_ENV"
if [ -f "$CLI_FRONTEND_ENV" ]; then
  load_env_file "$CLI_FRONTEND_ENV"
fi

: "${VALIDATOR_LLM_SCRIPT:?VALIDATOR_LLM_SCRIPT is required in .env.local}"
: "${VALIDATOR_LLM_API_KEY:?VALIDATOR_LLM_API_KEY is required in .env.local}"
if [ "$VALIDATOR_LLM_API_KEY" = "replace-with-your-real-validator-llm-key" ]; then
  echo "VALIDATOR_LLM_API_KEY is still the placeholder value in .env.local."
  echo "Replace it with your real validator LLM key before starting validator-service."
  exit 1
fi

export VALIDATOR_RPC_URL="${VALIDATOR_RPC_URL:-${VITE_RPC_URL:-http://127.0.0.1:8545}}"
export VALIDATOR_MARKET_ADDRESS="${VALIDATOR_MARKET_ADDRESS:-${VITE_MARKET_ADDRESS:-}}"
export VALIDATOR_PRIVATE_KEY="${VALIDATOR_PRIVATE_KEY:-0x7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6}"
export VALIDATOR_BASE_URL="${VALIDATOR_BASE_URL:-${VITE_VALIDATOR_API_BASE_URL:-http://localhost:8082}}"
export VALIDATOR_HTTP_ADDR="${VALIDATOR_HTTP_ADDR:-:8082}"
export VALIDATOR_LLM_TIMEOUT="${VALIDATOR_LLM_TIMEOUT:-60s}"

if [ -z "$VALIDATOR_MARKET_ADDRESS" ] || [ "$VALIDATOR_MARKET_ADDRESS" = "0x0000000000000000000000000000000000000000" ]; then
  echo "VALIDATOR_MARKET_ADDRESS is missing."
  echo "Set VALIDATOR_MARKET_ADDRESS in .env.local or VITE_MARKET_ADDRESS in cli-frontend/.env."
  exit 1
fi

echo "Starting validator-service with real LLM configuration"
echo "  RPC:    $VALIDATOR_RPC_URL"
echo "  Market: $VALIDATOR_MARKET_ADDRESS"
echo "  Base:   $VALIDATOR_BASE_URL"
echo "  Script: $VALIDATOR_LLM_SCRIPT"

cd "$ROOT/validator-service"
exec go run ./cmd/validator-service serve
