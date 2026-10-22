import { useState, type ReactNode } from 'react'
import { applyTheme, readStoredTheme, storeTheme, toggleTheme, type Theme } from '@/shared/domain/theme'
import { ThemeContext } from './ThemeContext'

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<Theme>(() => readStoredTheme(window.localStorage))

  function switchTheme() {
    const next = toggleTheme(theme)
    applyTheme(document.documentElement, next)
    storeTheme(window.localStorage, next)
    setTheme(next)
  }

  return <ThemeContext.Provider value={{ theme, toggleTheme: switchTheme }}>{children}</ThemeContext.Provider>
}
