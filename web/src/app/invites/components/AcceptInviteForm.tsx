import { useNavigate } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/app/auth/components/AuthContext'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { useErrorMessage } from '@/shared/hooks/useErrorMessage'
import { Button } from '@/shared/ui/button'
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from '@/shared/ui/field'
import { Input } from '@/shared/ui/input'
import { Spinner } from '@/shared/ui/spinner'
import { acceptInvitation } from '../datasource/invitesDatasource'
import { passwordsMatch } from '../domain/invite'

export function AcceptInviteForm({ secret, email }: { secret: string; email: string }) {
  const { t } = useTranslation()
  const { completeSignIn } = useAuth()
  const { describeFieldError } = useErrorMessage()
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [passwordConfirmation, setPasswordConfirmation] = useState('')
  const [mismatch, setMismatch] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const [submitting, setSubmitting] = useState(false)
  const nameError = describeFieldError(error, 'name')

  async function submitAcceptance(event: FormEvent) {
    event.preventDefault()
    setError(null)
    if (!passwordsMatch(password, passwordConfirmation)) {
      setMismatch(true)
      return
    }
    setMismatch(false)
    setSubmitting(true)
    try {
      const signIn = await acceptInvitation(secret, name, password)
      await completeSignIn(signIn.accessToken)
      await navigate({ to: '/' })
    } catch (caught) {
      setError(caught)
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={submitAcceptance} noValidate>
      <FieldGroup>
        <ErrorAlert error={error} />
        <Field>
          <FieldLabel htmlFor="accept-email">{t('acceptInvite.email')}</FieldLabel>
          <Input id="accept-email" type="email" value={email} readOnly disabled />
        </Field>
        <Field data-invalid={nameError ? true : undefined}>
          <FieldLabel htmlFor="accept-name">{t('acceptInvite.name')}</FieldLabel>
          <Input
            id="accept-name"
            autoComplete="name"
            required
            value={name}
            aria-invalid={nameError ? true : undefined}
            onChange={(event) => setName(event.target.value)}
          />
          {nameError ? <FieldError>{nameError}</FieldError> : null}
        </Field>
        <Field>
          <FieldLabel htmlFor="accept-password">{t('acceptInvite.password')}</FieldLabel>
          <Input
            id="accept-password"
            type="password"
            autoComplete="new-password"
            required
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
          <FieldDescription>{t('acceptInvite.passwordHint')}</FieldDescription>
        </Field>
        <Field data-invalid={mismatch ? true : undefined}>
          <FieldLabel htmlFor="accept-password-confirmation">{t('acceptInvite.passwordConfirmation')}</FieldLabel>
          <Input
            id="accept-password-confirmation"
            type="password"
            autoComplete="new-password"
            required
            value={passwordConfirmation}
            aria-invalid={mismatch ? true : undefined}
            onChange={(event) => setPasswordConfirmation(event.target.value)}
          />
          {mismatch ? <FieldError>{t('acceptInvite.passwordMismatch')}</FieldError> : null}
        </Field>
        <Button type="submit" disabled={submitting}>
          {submitting ? <Spinner /> : null}
          {submitting ? t('acceptInvite.submitting') : t('acceptInvite.submit')}
        </Button>
      </FieldGroup>
    </form>
  )
}
