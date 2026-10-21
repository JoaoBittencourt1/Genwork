import { http, HttpResponse, delay } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import { errorResponse, server } from '../../test/server'
import { ApiError, apiRequest, onSessionEnded, setAccessToken } from './client'

describe('apiRequest', () => {
  it('sends the bearer token and the web client header', async () => {
    let captured: Headers | undefined
    server.use(
      http.get('*/api/core/v1/me', ({ request }) => {
        captured = request.headers
        return HttpResponse.json({ ok: true })
      }),
    )
    setAccessToken('token-1')

    await apiRequest('GET', '/api/core/v1/me')

    expect(captured?.get('Authorization')).toBe('Bearer token-1')
    expect(captured?.get('X-Genwork-Client')).toBe('web')
  })

  it('omits the bearer token on unauthenticated requests', async () => {
    let authorization: string | null = 'unset'
    server.use(
      http.get('*/api/core/v1/invitations/abc', ({ request }) => {
        authorization = request.headers.get('Authorization')
        return HttpResponse.json({})
      }),
    )
    setAccessToken('token-1')

    await apiRequest('GET', '/api/core/v1/invitations/abc', { authenticated: false })

    expect(authorization).toBeNull()
  })

  it('refreshes once for concurrent expired requests and retries them', async () => {
    let refreshCalls = 0
    server.use(
      http.post('*/api/core/v1/auth/refresh', async () => {
        refreshCalls += 1
        await delay(20)
        return HttpResponse.json({ accessToken: 'fresh' })
      }),
      http.get('*/api/core/v1/me', ({ request }) =>
        request.headers.get('Authorization') === 'Bearer fresh'
          ? HttpResponse.json({ name: 'Ana' })
          : errorResponse(401, 'invalid_token'),
      ),
    )
    setAccessToken('expired')

    const results = await Promise.all([1, 2, 3].map(() => apiRequest<{ name: string }>('GET', '/api/core/v1/me')))

    expect(refreshCalls).toBe(1)
    expect(results.map((result) => result.name)).toEqual(['Ana', 'Ana', 'Ana'])
  })

  it('ends the session when refresh fails', async () => {
    server.use(http.get('*/api/core/v1/me', () => errorResponse(401, 'invalid_token')))
    const sessionEnded = vi.fn()
    onSessionEnded(sessionEnded)
    setAccessToken('expired')

    await expect(apiRequest('GET', '/api/core/v1/me')).rejects.toMatchObject({ code: 'invalid_token' })
    expect(sessionEnded).toHaveBeenCalledTimes(1)
  })

  it('does not refresh on other 401 errors', async () => {
    let refreshCalls = 0
    server.use(
      http.post('*/api/core/v1/auth/login', () => errorResponse(401, 'invalid_credentials')),
      http.post('*/api/core/v1/auth/refresh', () => {
        refreshCalls += 1
        return HttpResponse.json({ accessToken: 'fresh' })
      }),
    )

    const error = await apiRequest('POST', '/api/core/v1/auth/login', { body: {}, authenticated: true }).catch((caught) => caught)

    expect(error).toBeInstanceOf(ApiError)
    expect((error as ApiError).code).toBe('invalid_credentials')
    expect(refreshCalls).toBe(0)
  })

  it('exposes validation fields', async () => {
    server.use(http.post('*/api/core/v1/invites', () => errorResponse(400, 'validation_failed', { email: 'invalid' })))

    const error = (await apiRequest('POST', '/api/core/v1/invites', { body: {} }).catch((caught) => caught)) as ApiError

    expect(error.status).toBe(400)
    expect(error.fields).toEqual({ email: 'invalid' })
  })
})
