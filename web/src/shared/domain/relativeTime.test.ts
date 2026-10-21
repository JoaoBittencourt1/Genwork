import { describe, expect, it } from 'vitest'
import { formatTimeAgo } from './relativeTime'

const now = new Date('2026-10-21T12:00:00Z')

describe('formatTimeAgo', () => {
  it('picks the largest unit that fits', () => {
    expect(formatTimeAgo('2026-10-21T11:59:40Z', now, 'en')).toBe('now')
    expect(formatTimeAgo('2026-10-21T11:15:00Z', now, 'en')).toBe('45 minutes ago')
    expect(formatTimeAgo('2026-10-21T07:00:00Z', now, 'en')).toBe('5 hours ago')
    expect(formatTimeAgo('2026-10-20T12:00:00Z', now, 'en')).toBe('yesterday')
    expect(formatTimeAgo('2026-08-01T12:00:00Z', now, 'en')).toBe('2 months ago')
    expect(formatTimeAgo('2024-09-01T12:00:00Z', now, 'en')).toBe('2 years ago')
  })

  it('speaks the user language', () => {
    expect(formatTimeAgo('2026-10-18T12:00:00Z', now, 'pt-BR')).toBe('há 3 dias')
  })

  it('treats a timestamp slightly in the future as now', () => {
    expect(formatTimeAgo('2026-10-21T12:00:05Z', now, 'en')).toBe('now')
  })
})
