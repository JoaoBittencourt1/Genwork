import { describe, expect, it } from 'vitest'
import { mcpServerFixture } from '@/test/genagentsFixtures'
import { canDisconnectMcpOAuth, mcpOAuthReasonKey, mcpOAuthState, needsManualClient, readMcpOAuthResult } from './mcpOAuth'

const connection = { clientId: 'client-1', connectedAt: null, expiresAt: null, lastError: null }

describe('MCP server OAuth', () => {
  it('is available only to HTTP servers and follows the stored connection', () => {
    expect(mcpOAuthState(mcpServerFixture({ transport: 'stdio', url: null }))).toBe('unavailable')
    expect(mcpOAuthState(mcpServerFixture())).toBe('disconnected')
    expect(mcpOAuthState(mcpServerFixture({ oauth: { ...connection, status: 'connected' } }))).toBe('connected')
    expect(mcpOAuthState(mcpServerFixture({ oauth: { ...connection, status: 'error' } }))).toBe('error')
    expect(canDisconnectMcpOAuth(mcpServerFixture())).toBe(false)
    expect(canDisconnectMcpOAuth(mcpServerFixture({ oauth: { ...connection, status: 'pending' } }))).toBe(true)
  })

  it('reads the callback outcome from the page URL', () => {
    expect(readMcpOAuthResult('?mcpOAuth=connected')).toEqual({ connected: true, reason: null })
    expect(readMcpOAuthResult('?mcpOAuth=failed&reason=authorization_denied')).toEqual({ connected: false, reason: 'authorization_denied' })
    expect(readMcpOAuthResult('?other=1')).toBeNull()
  })

  it('translates known failure reasons and falls back for the rest', () => {
    expect(mcpOAuthReasonKey('invalid_state')).toBe('genagents.mcpServers.oauth.reasons.invalid_state')
    expect(mcpOAuthReasonKey('weird')).toBe('genagents.mcpServers.oauth.reasons.unknown')
    expect(mcpOAuthReasonKey(null)).toBe('genagents.mcpServers.oauth.reasons.unknown')
  })

  it('asks for a client id only when the server has no registration', () => {
    expect(needsManualClient('oauth_client_id_required')).toBe(true)
    expect(needsManualClient('oauth_discovery_failed')).toBe(false)
    expect(needsManualClient(null)).toBe(false)
  })
})
