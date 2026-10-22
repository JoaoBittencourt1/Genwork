import { useRouter, useSearch } from '@tanstack/react-router'
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { AuthLayout } from '@/shared/components/AuthLayout'
import { LoadingScreen } from '@/shared/components/LoadingScreen'
import { useInstanceName } from '@/shared/hooks/useInstanceName'
import { useAuth } from './components/AuthContext'
import { LoginForm } from './components/LoginForm'
import { safeRedirectTarget } from './domain/safeRedirectTarget'

export function LoginPage() {
  const { t } = useTranslation()
  const { state } = useAuth()
  const router = useRouter()
  const instanceName = useInstanceName() ?? t('app.name')
  const { redirect } = useSearch({ strict: false }) as { redirect?: unknown }
  const target = safeRedirectTarget(typeof redirect === 'string' ? redirect : undefined)

  // The history gets the target as written: an OAuth query must reach the consent page byte for byte.
  useEffect(() => {
    if (state.status === 'signedIn') router.history.replace(target)
  }, [state.status, router, target])

  if (state.status !== 'signedOut') return <LoadingScreen />

  return (
    <AuthLayout title={t('login.title')} description={t('login.subtitle', { instance: instanceName })}>
      <LoginForm />
    </AuthLayout>
  )
}
