import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { useErrorMessage } from '@/shared/hooks/useErrorMessage'
import { Button } from '@/shared/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/shared/ui/card'
import { Field, FieldError, FieldLabel } from '@/shared/ui/field'
import { Input } from '@/shared/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui/select'
import { Spinner } from '@/shared/ui/spinner'
import { createInvite, listRoles } from '../datasource/invitesDatasource'
import { pickDefaultRole, type InviteDelivery } from '../domain/invite'
import { invitesQueryKey } from './queryKeys'

export function InviteForm({ onDelivered }: { onDelivered: (delivery: InviteDelivery) => void }) {
  const { t } = useTranslation()
  const { describeFieldError } = useErrorMessage()
  const queryClient = useQueryClient()
  const roles = useQuery({ queryKey: ['roles'], queryFn: listRoles })
  const [email, setEmail] = useState('')
  const [selectedRole, setSelectedRole] = useState('')
  const roleToken = selectedRole || pickDefaultRole(roles.data ?? [])?.token || ''

  const invite = useMutation({
    mutationFn: () => createInvite(email, roleToken),
    onSuccess: async (delivery) => {
      setEmail('')
      onDelivered(delivery)
      await queryClient.invalidateQueries({ queryKey: invitesQueryKey })
    },
  })
  const emailError = describeFieldError(invite.error, 'email')

  function submitInvite(event: FormEvent) {
    event.preventDefault()
    invite.mutate()
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('invites.newInvite')}</CardTitle>
      </CardHeader>
      <CardContent>
        <form className="flex flex-col gap-4" onSubmit={submitInvite} noValidate>
          <ErrorAlert error={invite.error} />
          <div className="flex flex-col gap-4 sm:flex-row sm:items-end">
            <Field className="flex-1" data-invalid={emailError ? true : undefined}>
              <FieldLabel htmlFor="invite-email">{t('invites.email')}</FieldLabel>
              <Input
                id="invite-email"
                type="email"
                required
                value={email}
                aria-invalid={emailError ? true : undefined}
                onChange={(event) => setEmail(event.target.value)}
              />
              {emailError ? <FieldError>{emailError}</FieldError> : null}
            </Field>
            <Field className="sm:w-44">
              <FieldLabel htmlFor="invite-role">{t('invites.role')}</FieldLabel>
              <Select value={roleToken} onValueChange={setSelectedRole}>
                <SelectTrigger id="invite-role" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(roles.data ?? []).map((role) => (
                    <SelectItem key={role.token} value={role.token}>
                      {role.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
            <Button type="submit" disabled={invite.isPending || !roleToken}>
              {invite.isPending ? <Spinner /> : null}
              {invite.isPending ? t('invites.submitting') : t('invites.submit')}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}
