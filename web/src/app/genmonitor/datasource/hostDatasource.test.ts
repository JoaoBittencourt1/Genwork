import { HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { ApiError, setAccessToken } from '@/shared/datasource/httpClient'
import { hostSnapshotFixture } from '@/test/genmonitorFixtures'
import { captureRequests, errorResponse } from '@/test/server'
import { readHostSnapshot } from './hostDatasource'

describe('hostDatasource', () => {
  it('reads the latest host snapshot with the access token', async () => {
    const snapshot = hostSnapshotFixture()
    const requests = captureRequests('get', '*/api/genmonitor/v1/host', () => HttpResponse.json(snapshot))
    setAccessToken('token-1')

    await expect(readHostSnapshot()).resolves.toEqual(snapshot)
    expect(requests[0]?.headers.get('Authorization')).toBe('Bearer token-1')
  })

  it('reports a module that has not sampled the host yet', async () => {
    captureRequests('get', '*/api/genmonitor/v1/host', () => errorResponse(503, 'service_unavailable'))
    setAccessToken('token-1')

    await expect(readHostSnapshot()).rejects.toEqual(new ApiError(503, 'service_unavailable'))
  })
})
