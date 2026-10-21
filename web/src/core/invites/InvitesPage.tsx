import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { describeError, describeFieldError } from '../../shared/i18n/errors'
import { Alert } from '../../shared/ui/Alert'
import { Button } from '../../shared/ui/Button'
import { SelectField } from '../../shared/ui/SelectField'
import { TextField } from '../../shared/ui/TextField'
import { PERMISSION_INVITE_CREATE, PERMISSION_INVITE_READ, PERMISSION_INVITE_REVOKE, useAuth } from '../auth/AuthContext'
import { createInvite, listInvites, listRoles, resendInvite, revokeInvite, type Invite, type InviteDelivery } from './api'

const invitesQueryKey = ['invites']

export function InvitesPage() {
  const { t } = useTranslation()
  const { can } = useAuth()
  const [lastDelivery, setLastDelivery] = useState<InviteDelivery | null>(null)

  if (!can(PERMISSION_INVITE_READ)) {
    return (
      <section>
        <h1 className="text-2xl font-semibold">{t('invites.noPermissionTitle')}</h1>
        <p className="mt-2 text-slate-600">{t('invites.noPermissionDescription')}</p>
      </section>
    )
  }

  return (
    <section className="flex flex-col gap-8">
      <h1 className="text-2xl font-semibold">{t('invites.title')}</h1>
      {can(PERMISSION_INVITE_CREATE) ? <InviteForm onDelivered={setLastDelivery} /> : null}
      {lastDelivery ? <DeliveryPanel delivery={lastDelivery} /> : null}
      <InviteTable onDelivered={setLastDelivery} />
    </section>
  )
}

function InviteForm({ onDelivered }: { onDelivered: (delivery: InviteDelivery) => void }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const roles = useQuery({ queryKey: ['roles'], queryFn: listRoles })
  const [email, setEmail] = useState('')
  const [selectedRole, setSelectedRole] = useState('')
  const defaultRole = roles.data?.find((role) => role.name === 'member') ?? roles.data?.[0]
  const roleToken = selectedRole || defaultRole?.token || ''

  const invite = useMutation({
    mutationFn: () => createInvite(email, roleToken),
    onSuccess: async (delivery) => {
      setEmail('')
      onDelivered(delivery)
      await queryClient.invalidateQueries({ queryKey: invitesQueryKey })
    },
  })

  function submitInvite(event: FormEvent) {
    event.preventDefault()
    invite.mutate()
  }

  return (
    <form className="flex flex-col gap-4 rounded-xl border border-slate-200 bg-white p-5" onSubmit={submitInvite} noValidate>
      <h2 className="font-medium">{t('invites.newInvite')}</h2>
      {invite.isError ? <Alert tone="error">{describeError(t, invite.error)}</Alert> : null}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end">
        <TextField
          className="flex-1"
          label={t('invites.email')}
          type="email"
          required
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          error={describeFieldError(t, invite.error, 'email')}
        />
        <SelectField
          label={t('invites.role')}
          value={roleToken}
          onChange={(event) => setSelectedRole(event.target.value)}
          options={(roles.data ?? []).map((role) => ({ value: role.token, label: role.name }))}
        />
        <Button type="submit" disabled={invite.isPending || !roleToken}>
          {invite.isPending ? t('invites.submitting') : t('invites.submit')}
        </Button>
      </div>
    </form>
  )
}

function DeliveryPanel({ delivery }: { delivery: InviteDelivery }) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)

  async function copyLink() {
    await navigator.clipboard.writeText(delivery.link)
    setCopied(true)
  }

  return (
    <div className="flex flex-col gap-3 rounded-xl border border-blue-200 bg-blue-50 p-5">
      <h2 className="font-medium">{t('invites.linkTitle')}</h2>
      <p className="text-sm text-slate-600">{t('invites.linkDescription')}</p>
      <div className="flex flex-col gap-2 sm:flex-row">
        <input
          readOnly
          aria-label={t('invites.linkTitle')}
          value={delivery.link}
          className="flex-1 rounded-md border border-slate-300 bg-white px-3 py-2 font-mono text-xs"
        />
        <Button variant="secondary" onClick={() => void copyLink()}>
          {copied ? t('invites.copied') : t('invites.copy')}
        </Button>
      </div>
      {delivery.emailSent ? (
        <Alert tone="success">{t('invites.emailSent', { email: delivery.invite.email })}</Alert>
      ) : (
        <Alert tone="warning">{t('invites.emailNotSent')}</Alert>
      )}
    </div>
  )
}

function InviteTable({ onDelivered }: { onDelivered: (delivery: InviteDelivery) => void }) {
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
  const actionError = resend.error ?? revoke.error

  if (invites.isPending) return <p className="text-sm text-slate-500">{t('app.loading')}</p>
  if (invites.isError) return <Alert tone="error">{describeError(t, invites.error)}</Alert>
  if (invites.data.length === 0) return <p className="text-sm text-slate-500">{t('invites.empty')}</p>

  return (
    <div className="flex flex-col gap-3">
      {actionError ? <Alert tone="error">{describeError(t, actionError)}</Alert> : null}
      <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white">
        <table className="w-full text-left text-sm">
          <thead className="border-b border-slate-200 bg-slate-50 text-xs uppercase text-slate-500">
            <tr>
              <th className="px-4 py-2">{t('invites.columns.email')}</th>
              <th className="px-4 py-2">{t('invites.columns.role')}</th>
              <th className="px-4 py-2">{t('invites.columns.status')}</th>
              <th className="px-4 py-2">{t('invites.columns.invitedBy')}</th>
              <th className="px-4 py-2">{t('invites.columns.expiresAt')}</th>
              <th className="px-4 py-2">{t('invites.columns.actions')}</th>
            </tr>
          </thead>
          <tbody>
            {invites.data.map((invite) => (
              <tr key={invite.token} className="border-b border-slate-100 last:border-0">
                <td className="px-4 py-2">{invite.email}</td>
                <td className="px-4 py-2">{invite.role.name}</td>
                <td className="px-4 py-2">{t(`invites.status.${invite.status}`)}</td>
                <td className="px-4 py-2">{invite.invitedBy.name}</td>
                <td className="px-4 py-2">{formatDate.format(new Date(invite.expiresAt))}</td>
                <td className="flex gap-2 px-4 py-2">
                  {can(PERMISSION_INVITE_CREATE) && (invite.status === 'pending' || invite.status === 'expired') ? (
                    <Button variant="secondary" disabled={resend.isPending} onClick={() => resend.mutate(invite)}>
                      {t('invites.resend')}
                    </Button>
                  ) : null}
                  {can(PERMISSION_INVITE_REVOKE) && invite.status === 'pending' ? (
                    <Button variant="danger" disabled={revoke.isPending} onClick={() => revoke.mutate(invite)}>
                      {t('invites.revoke')}
                    </Button>
                  ) : null}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
