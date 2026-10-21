import { Link } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '../../shared/ui/Button'
import { useInstanceName } from '../discovery/useDiscovery'
import { PERMISSION_INVITE_READ, useAuth } from './AuthContext'

export function AppShell({ children }: { children: ReactNode }) {
  const { t } = useTranslation()
  const { state, signOut, can } = useAuth()
  const instanceName = useInstanceName()
  const linkClasses = 'rounded-md px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-100 [&.active]:bg-slate-100 [&.active]:font-medium [&.active]:text-slate-900'

  return (
    <div className="min-h-screen">
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-5xl items-center justify-between gap-4 px-4 py-3">
          <div className="flex items-center gap-6">
            <span className="font-semibold">{instanceName ?? t('app.name')}</span>
            <nav className="flex gap-1">
              <Link to="/" className={linkClasses} activeOptions={{ exact: true }}>
                {t('app.nav.home')}
              </Link>
              {can(PERMISSION_INVITE_READ) ? (
                <Link to="/invites" className={linkClasses}>
                  {t('app.nav.invites')}
                </Link>
              ) : null}
            </nav>
          </div>
          <div className="flex items-center gap-3">
            {state.status === 'signedIn' ? <span className="text-sm text-slate-600">{state.me.name}</span> : null}
            <Button variant="secondary" onClick={() => void signOut()}>
              {t('app.signOut')}
            </Button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-4 py-8">{children}</main>
    </div>
  )
}
