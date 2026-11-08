import { describe, expect, it } from 'vitest'
import { canFreeSpace, cleanupOutcomeKey } from './serverCleanup'

const image = { reference: 'ghcr.io/acme/genwork-core:2.2.3', size: 100 }

describe('server cleanup', () => {
  it('offers to free space only when earlier images remain', () => {
    expect(canFreeSpace(undefined)).toBe(false)
    expect(canFreeSpace({ images: [], totalBytes: 0 })).toBe(false)
    expect(canFreeSpace({ images: [image], totalBytes: 100 })).toBe(true)
  })

  it('says what a cleanup did', () => {
    expect(cleanupOutcomeKey({ removed: [], skipped: [], freedBytes: 0 })).toBe('system.cleanup.nothingRemoved')
    expect(cleanupOutcomeKey({ removed: [image], skipped: [], freedBytes: 100 })).toBe('system.cleanup.removed')
    expect(cleanupOutcomeKey({ removed: [image], skipped: [image], freedBytes: 100 })).toBe('system.cleanup.removedWithSkipped')
  })
})
