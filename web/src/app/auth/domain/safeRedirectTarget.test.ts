import { describe, expect, it } from 'vitest'
import { loginRedirectSearch, safeRedirectTarget } from './safeRedirectTarget'

describe('safeRedirectTarget', () => {
  it('accepts a path on this origin with its query and fragment', () => {
    expect(safeRedirectTarget('/oauth/authorize?client_id=abc&state=x%2Fy')).toBe('/oauth/authorize?client_id=abc&state=x%2Fy')
    expect(safeRedirectTarget('/tasks/board-1#task-2')).toBe('/tasks/board-1#task-2')
    expect(safeRedirectTarget('/')).toBe('/')
  })

  it.each([
    ['protocol-relative', '//evil.example.com'],
    ['absolute', 'https://evil.example.com'],
    ['javascript', 'javascript:alert(1)'],
    ['backslash', '/\\evil.example.com'],
    ['encoded backslash host', '/\\/evil.example.com'],
    ['tab inside the scheme', '/\t/evil.example.com'],
    ['relative without slash', 'evil.example.com'],
    ['login itself', '/login?redirect=/'],
    ['empty', ''],
    ['missing', undefined],
  ])('sends a %s target to the home page', (_, raw) => {
    expect(safeRedirectTarget(raw)).toBe('/')
  })
})

describe('loginRedirectSearch', () => {
  it('remembers the page that sent the user to the login', () => {
    expect(loginRedirectSearch('/oauth/authorize?client_id=abc')).toEqual({ redirect: '/oauth/authorize?client_id=abc' })
  })

  it('adds nothing for the home page', () => {
    expect(loginRedirectSearch('/')).toEqual({})
  })
})
