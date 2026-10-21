import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { useErrorMessage } from '@/shared/hooks/useErrorMessage'
import { Button } from '@/shared/ui/button'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/shared/ui/field'
import { Input } from '@/shared/ui/input'
import { Spinner } from '@/shared/ui/spinner'
import { useAuth } from './AuthContext'

export function LoginForm() {
  const { t } = useTranslation()
  const { signIn } = useAuth()
  const { describeFieldError } = useErrorMessage()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<unknown>(null)
  const [submitting, setSubmitting] = useState(false)
  const emailError = describeFieldError(error, 'email')

  async function submitLogin(event: FormEvent) {
    event.preventDefault()
    setSubmitting(true)
    setError(null)
    try {
      await signIn(email, password)
    } catch (caught) {
      setError(caught)
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={submitLogin} noValidate>
      <FieldGroup>
        <ErrorAlert error={error} />
        <Field data-invalid={emailError ? true : undefined}>
          <FieldLabel htmlFor="login-email">{t('login.email')}</FieldLabel>
          <Input
            id="login-email"
            type="email"
            autoComplete="email"
            required
            value={email}
            aria-invalid={emailError ? true : undefined}
            onChange={(event) => setEmail(event.target.value)}
          />
          {emailError ? <FieldError>{emailError}</FieldError> : null}
        </Field>
        <Field>
          <FieldLabel htmlFor="login-password">{t('login.password')}</FieldLabel>
          <Input
            id="login-password"
            type="password"
            autoComplete="current-password"
            required
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </Field>
        <Button type="submit" disabled={submitting}>
          {submitting ? <Spinner /> : null}
          {submitting ? t('login.submitting') : t('login.submit')}
        </Button>
      </FieldGroup>
    </form>
  )
}
