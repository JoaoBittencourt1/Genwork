import { ChevronsUpDownIcon, LogOutIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/shared/ui/dropdown-menu'
import { SidebarMenu, SidebarMenuButton, SidebarMenuItem, useSidebar } from '@/shared/ui/sidebar'
import { useAuth } from './AuthContext'

export function UserMenu() {
  const { t } = useTranslation()
  const { state, signOut } = useAuth()
  const { isMobile } = useSidebar()

  if (state.status !== 'signedIn') return null

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton size="lg">
              <div className="grid flex-1 text-left leading-tight">
                <span className="truncate font-medium">{state.me.name}</span>
                <span className="truncate text-xs text-muted-foreground">{state.me.email}</span>
              </div>
              <ChevronsUpDownIcon className="ml-auto" />
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent side={isMobile ? 'bottom' : 'right'} align="end" className="w-56">
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
      </SidebarMenuItem>
    </SidebarMenu>
  )
}
