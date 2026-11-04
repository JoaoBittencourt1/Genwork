import { Navigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/app/auth/components/AuthContext'
import { AuthLayout } from '@/shared/components/AuthLayout'
import { ForgotPasswordForm } from './components/ForgotPasswordForm'

export function ForgotPasswordPage() {
  const { t } = useTranslation()
  const { state } = useAuth()

  if (state.status === 'signedIn') return <Navigate to="/" />
  return (
    <AuthLayout title={t('forgotPassword.title')} description={t('forgotPassword.subtitle')}>
      <ForgotPasswordForm />
    </AuthLayout>
  )
}
