import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Separator } from '@/shared/ui/separator'
import { SidebarTrigger } from '@/shared/ui/sidebar'
import { ThemeToggle } from './ThemeToggle'

type PageHeaderProps = {
  children: ReactNode
  actions?: ReactNode
}

// PageHeader is the slim top bar of every signed-in page: sidebar toggle, title or breadcrumb, and page actions.
export function PageHeader({ children, actions }: PageHeaderProps) {
  const { t } = useTranslation()
  return (
    <header className="flex h-12 shrink-0 items-center gap-2 border-b px-3">
      <SidebarTrigger aria-label={t('app.toggleSidebar')} />
      <Separator orientation="vertical" className="mr-1 data-[orientation=vertical]:h-4" />
      <div className="flex min-w-0 flex-1 items-center gap-2 text-sm">{children}</div>
      {actions ? <div className="flex shrink-0 items-center gap-1">{actions}</div> : null}
      <ThemeToggle />
    </header>
  )
}
