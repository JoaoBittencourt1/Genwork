import { apiRequest } from '@/shared/datasource/httpClient'
import {
  toAuthorizationQuery,
  type AuthorizationDecision,
  type AuthorizationPreview,
  type AuthorizationRequest,
} from '../domain/authorizationRequest'

type PreviewResponse = { clientName?: string; scope?: string; redirectUrl?: string }

// previewAuthorization answers what the consent screen shows, or the redirect that reports the request's error to the
// client. Errors core will not redirect for (unknown client, unregistered redirect URI) are thrown and stay in Genwork.
export async function previewAuthorization(request: AuthorizationRequest): Promise<AuthorizationPreview> {
  const response = await apiRequest<PreviewResponse>('GET', `/api/core/v1/oauth/authorization-requests${toAuthorizationQuery(request)}`)
  if (response.redirectUrl) return { kind: 'redirect', redirectUrl: response.redirectUrl }
  return { kind: 'consent', clientName: response.clientName ?? '', scope: response.scope ?? '' }
}

export async function decideAuthorization(request: AuthorizationRequest, decision: AuthorizationDecision): Promise<string> {
  const response = await apiRequest<{ redirectUrl: string }>('POST', '/api/core/v1/oauth/authorizations', { body: { ...request, decision } })
  return response.redirectUrl
}
