import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'
import './index.css'
import { applyTheme, readStoredTheme } from '@/shared/domain/theme'
import { setUpI18n } from '@/shared/i18n'

setUpI18n()
applyTheme(document.documentElement, readStoredTheme(window.localStorage))

const container = document.getElementById('root')
if (!container) throw new Error('missing #root element')

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
