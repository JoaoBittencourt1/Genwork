import { describe, expect, it } from 'vitest'
import { applyTheme, readStoredTheme, storeTheme, THEME_STORAGE_KEY, toggleTheme, toTheme } from './theme'

const refusingStorage = {
  getItem: () => {
    throw new Error('storage disabled')
  },
  setItem: () => {
    throw new Error('storage disabled')
  },
}

describe('toTheme', () => {
  it.each([
    ['dark', 'dark'],
    ['light', 'light'],
    ['sepia', 'dark'],
    ['', 'dark'],
    [null, 'dark'],
  ])('maps %s to %s', (stored, expected) => {
    expect(toTheme(stored)).toBe(expected)
  })
})

describe('toggleTheme', () => {
  it('switches between dark and light', () => {
    expect(toggleTheme('dark')).toBe('light')
    expect(toggleTheme('light')).toBe('dark')
  })
})

describe('theme storage', () => {
  it('reads back what was stored', () => {
    storeTheme(window.localStorage, 'light')
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBe('light')
    expect(readStoredTheme(window.localStorage)).toBe('light')
    window.localStorage.removeItem(THEME_STORAGE_KEY)
  })

  it('falls back to dark when nothing is stored', () => {
    expect(readStoredTheme(window.localStorage)).toBe('dark')
  })

  it('survives a browser that refuses storage', () => {
    expect(readStoredTheme(refusingStorage)).toBe('dark')
    expect(() => storeTheme(refusingStorage, 'light')).not.toThrow()
  })
})

describe('applyTheme', () => {
  it('adds the dark class only for the dark theme', () => {
    const root = document.createElement('html')
    applyTheme(root, 'dark')
    expect(root.classList.contains('dark')).toBe(true)
    applyTheme(root, 'light')
    expect(root.classList.contains('dark')).toBe(false)
  })
})
