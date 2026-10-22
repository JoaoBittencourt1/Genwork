export const THEMES = ['dark', 'light'] as const
export type Theme = (typeof THEMES)[number]

export const DEFAULT_THEME: Theme = 'dark'

export const THEME_STORAGE_KEY = 'genwork.theme'

export function toTheme(value: string | null): Theme {
  return THEMES.find((theme) => theme === value) ?? DEFAULT_THEME
}

export function toggleTheme(theme: Theme): Theme {
  return theme === 'dark' ? 'light' : 'dark'
}

export function readStoredTheme(storage: Pick<Storage, 'getItem'>): Theme {
  try {
    return toTheme(storage.getItem(THEME_STORAGE_KEY))
  } catch {
    return DEFAULT_THEME
  }
}

// A browser that refuses storage (private mode, quota) still switches the theme for the current visit.
export function storeTheme(storage: Pick<Storage, 'setItem'>, theme: Theme): void {
  try {
    storage.setItem(THEME_STORAGE_KEY, theme)
  } catch {
    return
  }
}

export function applyTheme(root: Pick<HTMLElement, 'classList'>, theme: Theme): void {
  root.classList.toggle('dark', theme === 'dark')
}
