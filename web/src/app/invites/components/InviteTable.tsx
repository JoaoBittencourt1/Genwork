import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/app/auth/components/AuthContext'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { Badge } from '@/shared/ui/badge'
import { Button } from '@/shared/ui/button'
import { Spinner } from '@/shared/ui/spinner'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/ui/table'
import { listInvites, resendInvite, revokeInvite } from '../datasource/invitesDatasource'
import { inviteActions, type Invite, type InviteDelivery, type InviteStatus } from '../domain/invite'
import { invitesQueryKey } from './queryKeys'

const statusBadgeVariant: Record<InviteStatus, 'default' | 'secondary' | 'outline' | 'destructive'> = {
  pending: 'default',
  accepted: 'secondary',
  expired: 'outline',
  revoked: 'destructive',
}

export function InviteTable({ onDelivered }: { onDelivered: (delivery: InviteDelivery) => void }) {
  const { t, i18n } = useTranslation()
  const { can } = useAuth()
  const queryClient = useQueryClient()
  const invites = useQuery({ queryKey: invitesQueryKey, queryFn: listInvites })
  const formatDate = new Intl.DateTimeFormat(i18n.language, { dateStyle: 'short', timeStyle: 'short' })

  const resend = useMutation({
    mutationFn: (invite: Invite) => resendInvite(invite.token),
    onSuccess: async (delivery) => {
      onDelivered(delivery)
      await queryClient.invalidateQueries({ queryKey: invitesQueryKey })
    },
  })
  const revoke = useMutation({
    mutationFn: (invite: Invite) => revokeInvite(invite.token),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: invitesQueryKey }),
  })

  if (invites.isPending) return <Spinner />
  if (invites.isLoadingError) return <ErrorAlert error={invites.error} />
  if (invites.data.length === 0) return <p className="text-sm text-muted-foreground">{t('invites.empty')}</p>

  return (
    <div className="flex flex-col gap-3">
      <ErrorAlert error={resend.error ?? revoke.error} />
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('invites.columns.email')}</TableHead>
              <TableHead>{t('invites.columns.role')}</TableHead>
              <TableHead>{t('invites.columns.status')}</TableHead>
              <TableHead>{t('invites.columns.invitedBy')}</TableHead>
              <TableHead>{t('invites.columns.expiresAt')}</TableHead>
              <TableHead className="text-right">{t('invites.columns.actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {invites.data.map((invite) => {
              const actions = inviteActions(invite, can)
              return (
                <TableRow key={invite.token}>
                  <TableCell className="font-medium">{invite.email}</TableCell>
                  <TableCell>{invite.role.name}</TableCell>
                  <TableCell>
                    <Badge variant={statusBadgeVariant[invite.status]}>{t(`invites.status.${invite.status}`)}</Badge>
                  </TableCell>
                  <TableCell>{invite.invitedBy.name}</TableCell>
                  <TableCell>{formatDate.format(new Date(invite.expiresAt))}</TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-2">
                      {actions.canResend ? (
                        <Button variant="outline" size="sm" disabled={resend.isPending} onClick={() => resend.mutate(invite)}>
                          {t('invites.resend')}
                        </Button>
                      ) : null}
                      {actions.canRevoke ? (
                        <Button variant="destructive" size="sm" disabled={revoke.isPending} onClick={() => revoke.mutate(invite)}>
                          {t('invites.revoke')}
                        </Button>
                      ) : null}
                    </div>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
