import { Link } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { useErrorMessage } from '@/shared/hooks/useErrorMessage'
import { Button } from '@/shared/ui/button'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/shared/ui/field'
import { Input } from '@/shared/ui/input'
import { Spinner } from '@/shared/ui/spinner'
import { requestPasswordReset } from '../datasource/passwordResetDatasource'

export function ForgotPasswordForm() {
  const { t } = useTranslation()
  const { describeFieldError } = useErrorMessage()
  const [email, setEmail] = useState('')
  const [error, setError] = useState<unknown>(null)
  const [submitting, setSubmitting] = useState(false)
  const [sent, setSent] = useState(false)
  const emailError = describeFieldError(error, 'email')

  async function submitRequest(event: FormEvent) {
    event.preventDefault()
    setSubmitting(true)
    setError(null)
    try {
      await requestPasswordReset(email)
      setSent(true)
    } catch (caught) {
      setError(caught)
    } finally {
      setSubmitting(false)
    }
  }

  if (sent) {
    return (
      <FieldGroup>
        <p className="text-sm text-muted-foreground">{t('forgotPassword.sent')}</p>
        <Button asChild variant="outline" className="w-full">
          <Link to="/login">{t('forgotPassword.backToLogin')}</Link>
        </Button>
      </FieldGroup>
    )
  }

  return (
    <form onSubmit={submitRequest} noValidate>
      <FieldGroup>
        <ErrorAlert error={error} />
        <Field data-invalid={emailError ? true : undefined}>
          <FieldLabel htmlFor="forgot-email">{t('forgotPassword.email')}</FieldLabel>
          <Input
            id="forgot-email"
            type="email"
            autoComplete="email"
            required
            value={email}
            aria-invalid={emailError ? true : undefined}
            onChange={(event) => setEmail(event.target.value)}
          />
          {emailError ? <FieldError>{emailError}</FieldError> : null}
        </Field>
        <Button type="submit" disabled={submitting}>
          {submitting ? <Spinner /> : null}
          {submitting ? t('forgotPassword.submitting') : t('forgotPassword.submit')}
        </Button>
        <Button asChild variant="link" className="w-full">
          <Link to="/login">{t('forgotPassword.backToLogin')}</Link>
        </Button>
      </FieldGroup>
    </form>
  )
}
