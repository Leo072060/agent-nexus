/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_RPC_URL?: string
  readonly VITE_MARKET_ADDRESS?: string
  readonly VITE_VALIDATOR_API_BASE_URL?: string
  readonly VITE_DEFAULT_VIEW_AS?: string
  readonly VITE_POLL_MS?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
