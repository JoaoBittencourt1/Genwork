import { BlocksIcon, EllipsisIcon, Trash2Icon, UserCheckIcon, UserXIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/shared/components/ConfirmDialog'
import { Button } from '@/shared/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/shared/ui/dropdown-menu'
import type { Member, MemberActions, MemberStatus } from '../domain/member'

type MemberActionsMenuProps = {
  member: Member
  actions: MemberActions
  disabled: boolean
  onEditModules: () => void
  onChangeStatus: (status: MemberStatus) => void
  onRemove: () => void
}

type PendingConfirmation = 'deactivate' | 'remove' | null

export function MemberActionsMenu({ member, actions, disabled, onEditModules, onChangeStatus, onRemove }: MemberActionsMenuProps) {
  const { t } = useTranslation()
  const [confirming, setConfirming] = useState<PendingConfirmation>(null)
  const closeConfirmation = (open: boolean) => (open ? undefined : setConfirming(null))

  if (![actions.canChangeModules, actions.canDeactivate, actions.canReactivate, actions.canRemove].some(Boolean)) return null

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon-sm" disabled={disabled} aria-label={t('members.actionsFor', { name: member.name })}>
            <EllipsisIcon />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          {actions.canChangeModules ? (
            <DropdownMenuItem onSelect={onEditModules}>
              <BlocksIcon />
              {t('members.modules.manage')}
            </DropdownMenuItem>
          ) : null}
          {actions.canReactivate ? (
            <DropdownMenuItem onSelect={() => onChangeStatus('active')}>
              <UserCheckIcon />
              {t('members.reactivate')}
            </DropdownMenuItem>
          ) : null}
          {actions.canDeactivate ? (
            <DropdownMenuItem onSelect={() => setConfirming('deactivate')}>
              <UserXIcon />
              {t('members.deactivate')}
            </DropdownMenuItem>
          ) : null}
          {actions.canRemove ? (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem variant="destructive" onSelect={() => setConfirming('remove')}>
                <Trash2Icon />
                {t('members.remove')}
              </DropdownMenuItem>
            </>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
      <ConfirmDialog
        open={confirming === 'deactivate'}
        title={t('members.deactivateTitle', { name: member.name })}
        description={t('members.deactivateDescription', { name: member.name })}
        confirmLabel={t('members.deactivate')}
        onConfirm={() => onChangeStatus('deactivated')}
        onOpenChange={closeConfirmation}
      />
      <ConfirmDialog
        open={confirming === 'remove'}
        title={t('members.removeTitle', { name: member.name })}
        description={t('members.removeDescription', { name: member.name })}
        confirmLabel={t('members.remove')}
        onConfirm={onRemove}
        onOpenChange={closeConfirmation}
      />
    </>
  )
}
