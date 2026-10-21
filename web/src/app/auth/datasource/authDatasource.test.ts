import { HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { setAccessToken } from '@/shared/datasource/httpClient'
import { adminFixture, signInFixture } from '@/test/fixtures'
import { captureRequests, errorResponse } from '@/test/server'
import { fetchMe, login, logout, sessionGateway } from './authDatasource'

describe('authDatasource', () => {
  it('logs in as the web client without sending a bearer token', async () => {
    const requests = captureRequests('post', '*/api/core/v1/auth/login', () => HttpResponse.json(signInFixture))
    setAccessToken('stale-token')

    await expect(login('ana@example.com', 'correct horse')).resolves.toEqual(signInFixture)
    expect(requests[0]?.body).toEqual({ email: 'ana@example.com', password: 'correct horse', client: 'web' })
    expect(requests[0]?.headers.get('Authorization')).toBeNull()
  })

  it('surfaces login errors as API errors', async () => {
    captureRequests('post', '*/api/core/v1/auth/login', () => errorResponse(401, 'invalid_credentials'))
    await expect(login('ana@example.com', 'wrong')).rejects.toMatchObject({ status: 401, code: 'invalid_credentials' })
  })

  it('logs out with the current access token', async () => {
    const requests = captureRequests('post', '*/api/core/v1/auth/logout', () => new HttpResponse(null, { status: 204 }))
    setAccessToken('token-1')

    await expect(logout()).resolves.toBeUndefined()
    expect(requests[0]?.headers.get('Authorization')).toBe('Bearer token-1')
  })

  it('fetches the current user', async () => {
    captureRequests('get', '*/api/core/v1/me', () => HttpResponse.json(adminFixture))
    setAccessToken('token-1')
    await expect(fetchMe()).resolves.toEqual(adminFixture)
  })

  it('exposes refresh and profile loading as the session gateway', async () => {
    captureRequests('post', '*/api/core/v1/auth/refresh', () => HttpResponse.json(signInFixture))
    captureRequests('get', '*/api/core/v1/me', () => HttpResponse.json(adminFixture))

    await expect(sessionGateway.refresh()).resolves.toBe(true)
    await expect(sessionGateway.fetchMe()).resolves.toEqual(adminFixture)
  })
})
