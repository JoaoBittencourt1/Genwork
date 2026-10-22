// AuthorizationRequest is the query /oauth/authorize received; core validates it, the page only carries it.
export type AuthorizationRequest = {
  responseType: string
  clientId: string
  redirectUri: string
  state: string
  codeChallenge: string
  codeChallengeMethod: string
  scope: string
  resource: string
}

export type AuthorizationDecision = 'allow' | 'deny'

export type AuthorizationPreview = { kind: 'consent'; clientName: string; scope: string } | { kind: 'redirect'; redirectUrl: string }

const QUERY_NAMES: Record<keyof AuthorizationRequest, string> = {
  responseType: 'response_type',
  clientId: 'client_id',
  redirectUri: 'redirect_uri',
  state: 'state',
  codeChallenge: 'code_challenge',
  codeChallengeMethod: 'code_challenge_method',
  scope: 'scope',
  resource: 'resource',
}

// parseAuthorizationRequest is null without a client_id, since then there is nobody to ask consent for.
export function parseAuthorizationRequest(search: string): AuthorizationRequest | null {
  const query = new URLSearchParams(search)
  if (!query.get(QUERY_NAMES.clientId)) return null
  const request = {} as AuthorizationRequest
  for (const [field, name] of Object.entries(QUERY_NAMES) as [keyof AuthorizationRequest, string][]) {
    request[field] = query.get(name) ?? ''
  }
  return request
}

export function toAuthorizationQuery(request: AuthorizationRequest): string {
  const query = new URLSearchParams()
  for (const [field, name] of Object.entries(QUERY_NAMES) as [keyof AuthorizationRequest, string][]) {
    if (request[field]) query.set(name, request[field])
  }
  return `?${query.toString()}`
}
