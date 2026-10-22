import { describe, expect, it } from 'vitest'
import { hostSnapshotFixture } from '@/test/genmonitorFixtures'
import {
  formatBytes,
  formatBytesPerSecond,
  formatPercent,
  refreshIntervalMs,
  RETRY_INTERVAL_MS,
  uptimeParts,
  usageLevel,
  usedPercent,
} from './hostSnapshot'

describe('refreshIntervalMs', () => {
  it('follows the pace the module samples at', () => {
    expect(refreshIntervalMs(hostSnapshotFixture({ sampleIntervalSeconds: 5 }))).toBe(5000)
  })

  it('retries on its own pace before the first snapshot', () => {
    expect(refreshIntervalMs(undefined)).toBe(RETRY_INTERVAL_MS)
  })
})

describe('usedPercent', () => {
  it('divides used by used plus available', () => {
    expect(usedPercent(hostSnapshotFixture().memory)).toBe(25)
  })

  it('leaves blocks reserved for root out, like df', () => {
    expect(usedPercent({ totalBytes: 100, usedBytes: 45, availableBytes: 45 })).toBe(50)
  })

  it('is zero on an empty device', () => {
    expect(usedPercent({ totalBytes: 0, usedBytes: 0, availableBytes: 0 })).toBe(0)
  })
})

describe('usageLevel', () => {
  it('flags usage from 90%', () => {
    expect(usageLevel(89.9)).toBe('normal')
    expect(usageLevel(90)).toBe('high')
  })
})

describe('formatting', () => {
  it('formats percentages in the reader language', () => {
    expect(formatPercent(30.55, 'pt-BR')).toBe('30,6%')
    expect(formatPercent(30, 'en')).toBe('30%')
  })

  it('picks the largest binary unit below the value', () => {
    expect(formatBytes(512, 'en')).toBe('512 byte')
    expect(formatBytes(1536, 'en')).toBe('1.5 kB')
    expect(formatBytes(8589934592, 'pt-BR')).toBe('8 GB')
    expect(formatBytes(1024 ** 6, 'en')).toBe('1,024 PB')
  })

  it('formats throughput per second', () => {
    expect(formatBytesPerSecond(0, 'en')).toBe('0 byte/s')
    expect(formatBytesPerSecond(1572864, 'pt-BR')).toBe('1,5 MB/s')
  })
})

describe('uptimeParts', () => {
  it('splits uptime into days, hours, and minutes', () => {
    expect(uptimeParts(93602)).toEqual([
      { unit: 'days', count: 1 },
      { unit: 'hours', count: 2 },
      { unit: 'minutes', count: 0 },
    ])
  })

  it('drops leading zero units but always shows minutes', () => {
    expect(uptimeParts(3 * 3600 + 120)).toEqual([
      { unit: 'hours', count: 3 },
      { unit: 'minutes', count: 2 },
    ])
    expect(uptimeParts(30)).toEqual([{ unit: 'minutes', count: 0 }])
    expect(uptimeParts(-5)).toEqual([{ unit: 'minutes', count: 0 }])
  })
})
