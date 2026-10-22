import { createContext, useContext } from 'react'
import { DEFAULT_THEME, type Theme } from '@/shared/domain/theme'

export type ThemeContextValue = {
  theme: Theme
  toggleTheme: () => void
}

export const ThemeContext = createContext<ThemeContextValue>({ theme: DEFAULT_THEME, toggleTheme: () => undefined })

export function useTheme(): ThemeContextValue {
  return useContext(ThemeContext)
}
