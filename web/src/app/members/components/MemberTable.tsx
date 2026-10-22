import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useCapabilities } from '@/shared/components/CapabilitiesContext'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { UserAvatar } from '@/shared/components/UserAvatar'
import { moduleNameTranslationKey } from '@/shared/domain/capabilities'
import { formatTimeAgo } from '@/shared/domain/relativeTime'
import { assignableRoles } from '@/shared/domain/role'
import { useRoles } from '@/shared/hooks/useRoles'
import { cn } from '@/shared/lib/utils'
import { Badge } from '@/shared/ui/badge'
import { Spinner } from '@/shared/ui/spinner'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/ui/table'
import { listMembers, removeMember, updateMember } from '../datasource/membersDatasource'
import { memberActions, moduleAccessSummary, type Member, type MemberChanges, type MemberViewer } from '../domain/member'
import { MemberActionsMenu } from './MemberActionsMenu'
import { MemberModuleAccessDialog } from './MemberModuleAccessDialog'
import { MemberRoleSelect } from './MemberRoleSelect'
import { membersQueryKey } from './queryKeys'

type MemberUpdate = { member: Member; changes: MemberChanges }

export function MemberTable({ viewer }: { viewer: MemberViewer }) {
  const { t, i18n } = useTranslation()
  const { enabledModules } = useCapabilities()
  const queryClient = useQueryClient()
  const members = useQuery({ queryKey: membersQueryKey, queryFn: listMembers })
  const roles = useRoles(viewer.canManage)
  const [editingModulesOf, setEditingModulesOf] = useState<Member | null>(null)
  const refreshMembers = () => queryClient.invalidateQueries({ queryKey: membersQueryKey })

  const update = useMutation({
    mutationFn: ({ member, changes }: MemberUpdate) => updateMember(member.token, changes),
    onSuccess: refreshMembers,
  })
  const remove = useMutation({ mutationFn: (member: Member) => removeMember(member.token), onSuccess: refreshMembers })

  if (members.isPending) return <Spinner />
  if (members.isError) return <ErrorAlert error={members.error} />

  const roleList = roles.data ?? []
  const rolesToAssign = assignableRoles(roleList, viewer.rolePermissions)
  const enabledModuleIds = [...enabledModules].sort()
  const now = new Date()
  const busy = update.isPending || remove.isPending

  function describeModuleAccess(member: Member): string {
    const summary = moduleAccessSummary(enabledModuleIds, member.restrictedModules)
    if (summary.kind !== 'some') return t(`members.modules.${summary.kind}`)
    return summary.moduleIds.map((moduleId) => t(moduleNameTranslationKey(moduleId), { defaultValue: moduleId })).join(', ')
  }

  return (
    <div className="flex flex-col gap-3">
      <ErrorAlert error={update.error ?? remove.error} />
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('members.columns.member')}</TableHead>
              <TableHead>{t('members.columns.role')}</TableHead>
              <TableHead>{t('members.columns.status')}</TableHead>
              <TableHead>{t('members.columns.modules')}</TableHead>
              <TableHead>{t('members.columns.lastSignIn')}</TableHead>
              <TableHead className="text-right">
                <span className="sr-only">{t('members.columns.actions')}</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {members.data.map((member) => {
              const actions = memberActions(member, viewer, roleList)
              return (
                <TableRow key={member.token}>
                  <TableCell>
                    <div className={cn('flex items-center gap-3', member.status === 'deactivated' && 'opacity-60')}>
                      <UserAvatar name={member.name} avatarColor={member.avatarColor} />
                      <div className="flex min-w-0 flex-col">
                        <span className="flex items-center gap-2 truncate font-medium">
                          {member.name}
                          {actions.isSelf ? <Badge variant="outline">{t('members.you')}</Badge> : null}
                        </span>
                        <span className="truncate text-xs text-muted-foreground">{member.email}</span>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    {actions.canChangeRole ? (
                      <MemberRoleSelect
                        member={member}
                        roles={rolesToAssign}
                        disabled={busy}
                        onChange={(roleToken) => update.mutate({ member, changes: { role: roleToken } })}
                      />
                    ) : (
                      member.role.name
                    )}
                  </TableCell>
                  <TableCell>
                    <Badge variant={member.status === 'active' ? 'secondary' : 'outline'}>{t(`members.status.${member.status}`)}</Badge>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">{describeModuleAccess(member)}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {member.lastSignInAt ? formatTimeAgo(member.lastSignInAt, now, i18n.language) : t('members.neverSignedIn')}
                  </TableCell>
                  <TableCell>
                    <div className="flex justify-end">
                      <MemberActionsMenu
                        member={member}
                        actions={actions}
                        disabled={busy}
                        onEditModules={() => setEditingModulesOf(member)}
                        onChangeStatus={(status) => update.mutate({ member, changes: { status } })}
                        onRemove={() => remove.mutate(member)}
                      />
                    </div>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </div>
      <MemberModuleAccessDialog
        member={editingModulesOf}
        moduleIds={enabledModuleIds}
        saving={update.isPending}
        onSave={(member, restrictedModules) =>
          update.mutate({ member, changes: { restrictedModules } }, { onSuccess: () => setEditingModulesOf(null) })
        }
        onClose={() => setEditingModulesOf(null)}
      />
    </div>
  )
}
