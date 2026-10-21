import { Navigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { AuthLayout } from '@/shared/components/AuthLayout'
import { LoadingScreen } from '@/shared/components/LoadingScreen'
import { useInstanceName } from '@/shared/hooks/useInstanceName'
import { useAuth } from './components/AuthContext'
import { LoginForm } from './components/LoginForm'

export function LoginPage() {
  const { t } = useTranslation()
  const { state } = useAuth()
  const instanceName = useInstanceName() ?? t('app.name')

  if (state.status === 'loading') return <LoadingScreen />
  if (state.status === 'signedIn') return <Navigate to="/" />

  return (
    <AuthLayout title={t('login.title')} description={t('login.subtitle', { instance: instanceName })}>
      <LoginForm />
    </AuthLayout>
  )
}
