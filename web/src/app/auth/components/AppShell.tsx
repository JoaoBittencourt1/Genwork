import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { SidebarInset, SidebarProvider, SidebarTrigger } from '@/shared/ui/sidebar'
import { AppSidebar } from './AppSidebar'

export function AppShell({ children }: { children: ReactNode }) {
  const { t } = useTranslation()

  return (
    <SidebarProvider>
      <AppSidebar />
      <SidebarInset>
        <header className="flex h-14 items-center border-b px-4">
          <SidebarTrigger aria-label={t('app.toggleSidebar')} />
        </header>
        <div className="mx-auto w-full max-w-5xl px-4 py-8">{children}</div>
      </SidebarInset>
    </SidebarProvider>
  )
}
