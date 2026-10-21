import { apiRequest, refreshSession } from '@/shared/datasource/httpClient'
import type { Me, SessionGateway } from '../domain/session'

export type SignInResponse = {
  accessToken: string
  accessTokenExpiresAt: string
  session: string
}

export function login(email: string, password: string): Promise<SignInResponse> {
  return apiRequest<SignInResponse>('POST', '/api/core/v1/auth/login', {
    body: { email, password, client: 'web' },
    authenticated: false,
  })
}

export function logout(): Promise<void> {
  return apiRequest<void>('POST', '/api/core/v1/auth/logout')
}

export function fetchMe(): Promise<Me> {
  return apiRequest<Me>('GET', '/api/core/v1/me')
}

export const sessionGateway: SessionGateway = { refresh: refreshSession, fetchMe }
