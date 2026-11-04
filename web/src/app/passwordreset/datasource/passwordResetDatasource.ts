import { apiRequest } from '@/shared/datasource/httpClient'

export function requestPasswordReset(email: string): Promise<void> {
  return apiRequest<void>('POST', '/api/core/v1/auth/password-reset', { body: { email }, authenticated: false })
}

export function confirmPasswordReset(secret: string, password: string): Promise<void> {
  return apiRequest<void>('POST', `/api/core/v1/auth/password-reset/${encodeURIComponent(secret)}`, {
    body: { password },
    authenticated: false,
  })
}
