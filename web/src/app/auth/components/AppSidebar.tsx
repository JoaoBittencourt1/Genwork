import { Link, useMatchRoute } from '@tanstack/react-router'
import { HomeIcon, UsersIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useInstanceName } from '@/shared/hooks/useInstanceName'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from '@/shared/ui/sidebar'
import { PERMISSIONS } from '../domain/session'
import { useAuth } from './AuthContext'
import { UserMenu } from './UserMenu'

export function AppSidebar() {
  const { t } = useTranslation()
  const { can } = useAuth()
  const { setOpenMobile } = useSidebar()
  const matchRoute = useMatchRoute()
  const instanceName = useInstanceName() ?? t('app.name')

  const navigationEntries = [
    { to: '/', label: t('app.nav.home'), icon: HomeIcon, visible: true },
    { to: '/members', label: t('app.nav.members'), icon: UsersIcon, visible: can(PERMISSIONS.inviteRead) },
  ] as const

  return (
    <Sidebar collapsible="icon" mobileTitle={t('app.navigation')}>
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" asChild>
              <Link to="/" onClick={() => setOpenMobile(false)}>
                <img src="/icon-180.png" alt="" className="size-8 rounded-md" />
                <span className="truncate font-semibold">{instanceName}</span>
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              {navigationEntries
                .filter((entry) => entry.visible)
                .map((entry) => (
                  <SidebarMenuItem key={entry.to}>
                    <SidebarMenuButton asChild tooltip={entry.label} isActive={Boolean(matchRoute({ to: entry.to }))}>
                      <Link to={entry.to} onClick={() => setOpenMobile(false)}>
                        <entry.icon />
                        <span>{entry.label}</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarFooter>
        <UserMenu />
      </SidebarFooter>
    </Sidebar>
  )
}
