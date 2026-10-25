import { describe, expect, it } from 'vitest'
import { LIVE_REFRESH_INTERVAL_MS, liveRefreshInterval, NO_LIVE_REFRESH } from './liveRefresh'

describe('live refresh', () => {
  it('refreshes every 10 seconds', () => {
    expect(LIVE_REFRESH_INTERVAL_MS).toBe(10_000)
    expect(liveRefreshInterval({ savingChanges: false })).toBe(LIVE_REFRESH_INTERVAL_MS)
  })

  it('pauses while changes are being saved', () => {
    expect(liveRefreshInterval({ savingChanges: true })).toBe(false)
  })

  it('turns both the interval and the focus refresh off for opted-out queries', () => {
    expect(NO_LIVE_REFRESH).toEqual({ refetchInterval: false, refetchOnWindowFocus: false })
  })
})
