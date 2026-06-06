import { createPublicClient, http } from 'viem'
import { config } from '../config'

// Read-only client — no wallet/private key needed for any of the dashboard reads.
export const publicClient = createPublicClient({
  transport: http(config.rpcUrl),
})
