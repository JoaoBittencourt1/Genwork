import type { AuthorizationRequest } from '@/app/assistants/domain/authorizationRequest'

// claudeAuthorizationQuery is the query claude.ai sends to /oauth/authorize.
export const claudeAuthorizationQuery =
  '?response_type=code&client_id=client-claude&redirect_uri=https%3A%2F%2Fclaude.ai%2Fapi%2Fmcp%2Fauth_callback' +
  '&state=state-from-claude&code_challenge=E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM&code_challenge_method=S256' +
  '&scope=genwork&resource=https%3A%2F%2Fgenwork.example.com%2Fmcp'

export const claudeAuthorizationRequest: AuthorizationRequest = {
  responseType: 'code',
  clientId: 'client-claude',
  redirectUri: 'https://claude.ai/api/mcp/auth_callback',
  state: 'state-from-claude',
  codeChallenge: 'E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM',
  codeChallengeMethod: 'S256',
  scope: 'genwork',
  resource: 'https://genwork.example.com/mcp',
}

export const claudeCallbackWithCode = 'https://claude.ai/api/mcp/auth_callback?code=code-123&iss=https%3A%2F%2Fgenwork.example.com&state=state-from-claude'
export const claudeCallbackDenied = 'https://claude.ai/api/mcp/auth_callback?error=access_denied&iss=https%3A%2F%2Fgenwork.example.com&state=state-from-claude'
