import { describe, expect, it } from 'vitest'
import { formatBytes, formatBytesPerSecond } from './byteUnits'

describe('byte units', () => {
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
