import { useNavigate } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { passwordsMatch } from '@/shared/domain/password'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { Button } from '@/shared/ui/button'
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from '@/shared/ui/field'
import { Input } from '@/shared/ui/input'
import { Spinner } from '@/shared/ui/spinner'
import { confirmPasswordReset } from '../datasource/passwordResetDatasource'

export function ResetPasswordForm({ secret }: { secret: string }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [password, setPassword] = useState('')
  const [passwordConfirmation, setPasswordConfirmation] = useState('')
  const [mismatch, setMismatch] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const [submitting, setSubmitting] = useState(false)

  async function submitReset(event: FormEvent) {
    event.preventDefault()
    setError(null)
    if (!passwordsMatch(password, passwordConfirmation)) {
      setMismatch(true)
      return
    }
    setMismatch(false)
    setSubmitting(true)
    try {
      await confirmPasswordReset(secret, password)
      await navigate({ to: '/login' })
    } catch (caught) {
      setError(caught)
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={submitReset} noValidate>
      <FieldGroup>
        <ErrorAlert error={error} />
        <Field>
          <FieldLabel htmlFor="reset-password">{t('resetPassword.password')}</FieldLabel>
          <Input
            id="reset-password"
            type="password"
            autoComplete="new-password"
            required
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
          <FieldDescription>{t('resetPassword.passwordHint')}</FieldDescription>
        </Field>
        <Field data-invalid={mismatch ? true : undefined}>
          <FieldLabel htmlFor="reset-password-confirmation">{t('resetPassword.passwordConfirmation')}</FieldLabel>
          <Input
            id="reset-password-confirmation"
            type="password"
            autoComplete="new-password"
            required
            value={passwordConfirmation}
            aria-invalid={mismatch ? true : undefined}
            onChange={(event) => setPasswordConfirmation(event.target.value)}
          />
          {mismatch ? <FieldError>{t('resetPassword.passwordMismatch')}</FieldError> : null}
        </Field>
        <Button type="submit" disabled={submitting}>
          {submitting ? <Spinner /> : null}
          {submitting ? t('resetPassword.submitting') : t('resetPassword.submit')}
        </Button>
      </FieldGroup>
    </form>
  )
}
