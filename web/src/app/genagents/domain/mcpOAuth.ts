import type { McpServer } from './types'

// McpOAuthState is what the MCP server row shows about its OAuth connection.
export type McpOAuthState = 'unavailable' | 'disconnected' | 'connected' | 'error'

export function mcpOAuthState(server: McpServer): McpOAuthState {
  if (server.transport !== 'http') return 'unavailable'
  return server.oauth?.status ?? 'disconnected'
}

export function canDisconnectMcpOAuth(server: McpServer): boolean {
  return server.oauth !== null
}

export type McpOAuthResult = { connected: boolean; reason: string | null }

const KNOWN_OAUTH_REASONS = ['invalid_state', 'authorization_denied', 'token_exchange_failed', 'refresh_failed'] as const

// readMcpOAuthResult reads the outcome the OAuth callback puts in the MCP servers page URL.
export function readMcpOAuthResult(search: string): McpOAuthResult | null {
  const params = new URLSearchParams(search)
  const outcome = params.get('mcpOAuth')
  if (outcome === 'connected') return { connected: true, reason: null }
  if (outcome === 'failed') return { connected: false, reason: params.get('reason') }
  return null
}

// mcpOAuthReasonKey is the translation key of a failure reason, falling back to a generic message.
export function mcpOAuthReasonKey(reason: string | null): string {
  return reason !== null && (KNOWN_OAUTH_REASONS as readonly string[]).includes(reason)
    ? `genagents.mcpServers.oauth.reasons.${reason}`
    : 'genagents.mcpServers.oauth.reasons.unknown'
}

// needsManualClient says the authorization server has no dynamic registration, so the admin must enter a client id.
export function needsManualClient(errorCode: string | null): boolean {
  return errorCode === 'oauth_client_id_required'
}
