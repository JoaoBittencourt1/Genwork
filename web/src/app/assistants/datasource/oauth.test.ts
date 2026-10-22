import { HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { claudeAuthorizationRequest, claudeCallbackDenied, claudeCallbackWithCode } from '@/test/assistantFixtures'
import { captureRequests, errorResponse } from '@/test/server'
import { decideAuthorization, previewAuthorization } from './oauth'

describe('oauth datasource', () => {
  it('asks core about the request with its OAuth query', async () => {
    const requests = captureRequests('get', '*/api/core/v1/oauth/authorization-requests', () =>
      HttpResponse.json({ clientName: 'Claude', scope: 'genwork' }),
    )

    await expect(previewAuthorization(claudeAuthorizationRequest)).resolves.toEqual({ kind: 'consent', clientName: 'Claude', scope: 'genwork' })
    const query = new URLSearchParams(requests[0]?.search)
    expect(query.get('client_id')).toBe('client-claude')
    expect(query.get('redirect_uri')).toBe('https://claude.ai/api/mcp/auth_callback')
    expect(query.get('code_challenge_method')).toBe('S256')
  })

  it('tells a request core sends back to the client', async () => {
    captureRequests('get', '*/api/core/v1/oauth/authorization-requests', () => HttpResponse.json({ redirectUrl: claudeCallbackDenied }))

    await expect(previewAuthorization(claudeAuthorizationRequest)).resolves.toEqual({ kind: 'redirect', redirectUrl: claudeCallbackDenied })
  })

  it.each(['invalid_client', 'invalid_redirect_uri'])('surfaces %s, which stays in Genwork', async (code) => {
    captureRequests('get', '*/api/core/v1/oauth/authorization-requests', () => errorResponse(400, code))

    await expect(previewAuthorization(claudeAuthorizationRequest)).rejects.toMatchObject({ status: 400, code })
  })

  it.each([
    ['allow', claudeCallbackWithCode],
    ['deny', claudeCallbackDenied],
  ] as const)('posts the %s decision and answers where to send the browser', async (decision, redirectUrl) => {
    const requests = captureRequests('post', '*/api/core/v1/oauth/authorizations', () => HttpResponse.json({ redirectUrl }))

    await expect(decideAuthorization(claudeAuthorizationRequest, decision)).resolves.toBe(redirectUrl)
    expect(requests[0]?.body).toEqual({ ...claudeAuthorizationRequest, decision })
  })
})
