import { HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { capabilitiesFixture, usersFixture } from '@/test/gentasksFixtures'
import { captureRequests, errorResponse } from '@/test/server'
import { fetchCapabilities } from './capabilities'
import { setAccessToken } from './httpClient'
import { listUsers } from './users'

describe('core directory and capabilities', () => {
  it('fetches capabilities with the access token', async () => {
    const requests = captureRequests('get', '*/api/core/v1/capabilities', () => HttpResponse.json(capabilitiesFixture))
    setAccessToken('token-1')

    await expect(fetchCapabilities()).resolves.toEqual(capabilitiesFixture)
    expect(requests[0]?.headers.get('Authorization')).toBe('Bearer token-1')
  })

  it('lists directory users from their envelope', async () => {
    captureRequests('get', '*/api/core/v1/users', () => HttpResponse.json({ users: usersFixture }))
    await expect(listUsers()).resolves.toEqual(usersFixture)
  })

  it('surfaces a forbidden directory', async () => {
    captureRequests('get', '*/api/core/v1/users', () => errorResponse(403, 'forbidden'))
    await expect(listUsers()).rejects.toMatchObject({ code: 'forbidden' })
  })
})
