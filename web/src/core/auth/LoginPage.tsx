import { Navigate } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { describeError, describeFieldError } from '../../shared/i18n/errors'
import { Alert } from '../../shared/ui/Alert'
import { Button } from '../../shared/ui/Button'
import { CenteredCard } from '../../shared/ui/CenteredCard'
import { LoadingScreen } from '../../shared/ui/LoadingScreen'
import { TextField } from '../../shared/ui/TextField'
import { useInstanceName } from '../discovery/useDiscovery'
import { useAuth } from './AuthContext'

export function LoginPage() {
  const { t } = useTranslation()
  const { state, signIn } = useAuth()
  const instanceName = useInstanceName() ?? t('app.name')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<unknown>(null)
  const [submitting, setSubmitting] = useState(false)

  if (state.status === 'loading') return <LoadingScreen />
  if (state.status === 'signedIn') return <Navigate to="/" />

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
    <CenteredCard title={t('login.title')} subtitle={t('login.subtitle', { instance: instanceName })}>
      <form className="flex flex-col gap-4" onSubmit={submitLogin} noValidate>
        {error ? <Alert tone="error">{describeError(t, error)}</Alert> : null}
        <TextField
          label={t('login.email')}
          type="email"
          autoComplete="email"
          required
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          error={describeFieldError(t, error, 'email')}
        />
        <TextField
          label={t('login.password')}
          type="password"
          autoComplete="current-password"
          required
          value={password}
          onChange={(event) => setPassword(event.target.value)}
        />
        <Button type="submit" disabled={submitting}>
          {submitting ? t('login.submitting') : t('login.submit')}
        </Button>
      </form>
    </CenteredCard>
  )
}
