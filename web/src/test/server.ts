import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'
import type { Me } from '../core/auth/api'
import { discoveryFixture } from './fixtures'

export const server = setupServer(
  http.get('*/.well-known/genwork', () => HttpResponse.json(discoveryFixture)),
  http.post('*/api/core/v1/auth/refresh', () => HttpResponse.json({ error: { code: 'invalid_token' } }, { status: 401 })),
)

export function errorResponse(status: number, code: string, fields?: Record<string, string>) {
  return HttpResponse.json({ error: { code, fields } }, { status })
}

export function useSignedInSession(me: Me) {
  server.use(
    http.post('*/api/core/v1/auth/refresh', () =>
      HttpResponse.json({ accessToken: 'access-token', accessTokenExpiresAt: '2026-10-20T12:15:00Z', session: 'session-token' }),
    ),
    http.get('*/api/core/v1/me', () => HttpResponse.json(me)),
  )
}
