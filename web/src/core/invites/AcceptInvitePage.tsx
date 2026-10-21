import { Link, Navigate, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { describeError, describeFieldError } from '../../shared/i18n/errors'
import { Alert } from '../../shared/ui/Alert'
import { Button } from '../../shared/ui/Button'
import { CenteredCard } from '../../shared/ui/CenteredCard'
import { LoadingScreen } from '../../shared/ui/LoadingScreen'
import { TextField } from '../../shared/ui/TextField'
import { useAuth } from '../auth/AuthContext'
import { acceptInvitation, inspectInvitation } from './api'

export function AcceptInvitePage({ secret }: { secret: string }) {
  const { t } = useTranslation()
  const { state } = useAuth()
  const invitation = useQuery({
    queryKey: ['invitation', secret],
    queryFn: () => inspectInvitation(secret),
    retry: false,
  })

  if (state.status === 'signedIn') return <Navigate to="/" />
  if (invitation.isPending) return <LoadingScreen />
  if (invitation.isError) {
    return (
      <CenteredCard title={t('acceptInvite.notFoundTitle')}>
        <p className="text-sm text-slate-600">{t('acceptInvite.notFoundDescription')}</p>
        <Link to="/login" className="mt-4 inline-block text-sm font-medium text-blue-700 hover:underline">
          {t('acceptInvite.goToLogin')}
        </Link>
      </CenteredCard>
    )
  }
  return <AcceptInviteForm secret={secret} instanceName={invitation.data.instanceName} email={invitation.data.email} />
}

function AcceptInviteForm({ secret, instanceName, email }: { secret: string; instanceName: string; email: string }) {
  const { t } = useTranslation()
  const { completeSignIn } = useAuth()
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [passwordConfirmation, setPasswordConfirmation] = useState('')
  const [mismatch, setMismatch] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const [submitting, setSubmitting] = useState(false)

  async function submitAcceptance(event: FormEvent) {
    event.preventDefault()
    setError(null)
    if (password !== passwordConfirmation) {
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
    <CenteredCard title={t('acceptInvite.title')} subtitle={t('acceptInvite.subtitle', { instance: instanceName })}>
      <form className="flex flex-col gap-4" onSubmit={submitAcceptance} noValidate>
        {error ? <Alert tone="error">{describeError(t, error)}</Alert> : null}
        <TextField label={t('acceptInvite.email')} type="email" value={email} readOnly />
        <TextField
          label={t('acceptInvite.name')}
          autoComplete="name"
          required
          value={name}
          onChange={(event) => setName(event.target.value)}
          error={describeFieldError(t, error, 'name')}
        />
        <TextField
          label={t('acceptInvite.password')}
          type="password"
          autoComplete="new-password"
          required
          hint={t('acceptInvite.passwordHint')}
          value={password}
          onChange={(event) => setPassword(event.target.value)}
        />
        <TextField
          label={t('acceptInvite.passwordConfirmation')}
          type="password"
          autoComplete="new-password"
          required
          value={passwordConfirmation}
          onChange={(event) => setPasswordConfirmation(event.target.value)}
          error={mismatch ? t('acceptInvite.passwordMismatch') : undefined}
        />
        <Button type="submit" disabled={submitting}>
          {submitting ? t('acceptInvite.submitting') : t('acceptInvite.submit')}
        </Button>
      </form>
    </CenteredCard>
  )
}
