import { HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { setAccessToken } from '@/shared/datasource/httpClient'
import { availableUpdateFixture, updateStatusFixture, updatingFixture } from '@/test/systemFixtures'
import { captureRequests, errorResponse } from '@/test/server'
import { checkForUpdates, findUpdateStatus, startUpdate } from './updates'

describe('updates datasource', () => {
  it('reads the update status', async () => {
    const requests = captureRequests('get', '*/api/core/v1/system/update', () => HttpResponse.json(updateStatusFixture()))
    setAccessToken('token-1')

    await expect(findUpdateStatus()).resolves.toEqual(updateStatusFixture())
    expect(requests[0]?.headers.get('Authorization')).toBe('Bearer token-1')
  })

  it('checks for updates', async () => {
    const requests = captureRequests('post', '*/api/core/v1/system/update/check', () => HttpResponse.json(availableUpdateFixture))

    await expect(checkForUpdates()).resolves.toEqual(availableUpdateFixture)
    expect(requests[0]?.body).toBeUndefined()
  })

  it('starts the update of exactly the confirmed version', async () => {
    const requests = captureRequests('post', '*/api/core/v1/system/update', () => HttpResponse.json(updatingFixture, { status: 202 }))

    await expect(startUpdate('0.5.2')).resolves.toEqual(updatingFixture)
    expect(requests[0]?.body).toEqual({ version: '0.5.2' })
  })

  it.each([
    [409, 'version_mismatch'],
    [409, 'update_in_progress'],
    [409, 'no_newer_release'],
    [502, 'release_channel_unreachable'],
    [503, 'updater_unavailable'],
    [403, 'forbidden'],
  ])('surfaces %s %s', async (status, code) => {
    captureRequests('post', '*/api/core/v1/system/update', () => errorResponse(status, code))
    captureRequests('post', '*/api/core/v1/system/update/check', () => errorResponse(status, code))

    await expect(startUpdate('0.5.2')).rejects.toMatchObject({ status, code })
    await expect(checkForUpdates()).rejects.toMatchObject({ status, code })
  })
})
