import { Link, useMatchRoute } from '@tanstack/react-router'
import { HomeIcon, KanbanSquareIcon, UsersIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { GENTASKS_PERMISSIONS } from '@/app/gentasks/domain/permissions'
import { useCapabilities } from '@/shared/components/CapabilitiesContext'
import { isModuleEnabled, MODULE_IDS } from '@/shared/domain/capabilities'
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
  const { enabledModules } = useCapabilities()
  const { setOpenMobile } = useSidebar()
  const matchRoute = useMatchRoute()
  const instanceName = useInstanceName() ?? t('app.name')

  const navigationEntries = [
    { to: '/', label: t('app.nav.home'), icon: HomeIcon, visible: true, matchesNested: false },
    {
      to: '/tasks',
      label: t('app.nav.tasks'),
      icon: KanbanSquareIcon,
      visible: isModuleEnabled(enabledModules, MODULE_IDS.gentasks) && can(GENTASKS_PERMISSIONS.boardRead),
      matchesNested: true,
    },
    {
      to: '/members',
      label: t('app.nav.members'),
      icon: UsersIcon,
      visible: can(PERMISSIONS.inviteRead) || can(PERMISSIONS.memberRead),
      matchesNested: true,
    },
  ] as const

  return (
    <Sidebar collapsible="icon" mobileTitle={t('app.navigation')}>
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" asChild>
              <Link to="/" onClick={() => setOpenMobile(false)}>
                <img src="/icon-180.png" alt="" className="size-8" />
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
                    <SidebarMenuButton asChild tooltip={entry.label} isActive={Boolean(matchRoute({ to: entry.to, fuzzy: entry.matchesNested }))}>
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
