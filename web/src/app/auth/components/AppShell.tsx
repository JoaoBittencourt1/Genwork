import { Link } from '@tanstack/react-router'
import { ChevronDownIcon, LogOutIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useInstanceName } from '@/shared/hooks/useInstanceName'
import { Button } from '@/shared/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/shared/ui/dropdown-menu'
import { PERMISSIONS } from '../domain/session'
import { useAuth } from './AuthContext'

const navLinkClasses =
  'rounded-md px-3 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground [&.active]:bg-muted [&.active]:font-medium [&.active]:text-foreground'

export function AppShell({ children }: { children: ReactNode }) {
  const { t } = useTranslation()
  const { state, signOut, can } = useAuth()
  const instanceName = useInstanceName()

  return (
    <div className="min-h-svh bg-background">
      <header className="border-b">
        <div className="mx-auto flex h-14 max-w-5xl items-center justify-between gap-4 px-4">
          <div className="flex items-center gap-6">
            <span className="font-semibold">{instanceName ?? t('app.name')}</span>
            <nav className="flex gap-1">
              <Link to="/" className={navLinkClasses} activeOptions={{ exact: true }}>
                {t('app.nav.home')}
              </Link>
              {can(PERMISSIONS.inviteRead) ? (
                <Link to="/invites" className={navLinkClasses}>
                  {t('app.nav.invites')}
                </Link>
              ) : null}
            </nav>
          </div>
          {state.status === 'signedIn' ? (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="sm">
                  {state.me.name}
                  <ChevronDownIcon />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-56">
                <DropdownMenuLabel className="flex flex-col">
                  <span>{state.me.name}</span>
                  <span className="text-xs font-normal text-muted-foreground">{state.me.email}</span>
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem onSelect={() => void signOut()}>
                  <LogOutIcon />
                  {t('app.signOut')}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ) : null}
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-4 py-8">{children}</main>
    </div>
  )
}
