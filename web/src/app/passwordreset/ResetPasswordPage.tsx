import { Navigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/app/auth/components/AuthContext'
import { AuthLayout } from '@/shared/components/AuthLayout'
import { ResetPasswordForm } from './components/ResetPasswordForm'

export function ResetPasswordPage({ secret }: { secret: string }) {
  const { t } = useTranslation()
  const { state } = useAuth()

  if (state.status === 'signedIn') return <Navigate to="/" />
  return (
    <AuthLayout title={t('resetPassword.title')} description={t('resetPassword.subtitle')}>
      <ResetPasswordForm secret={secret} />
    </AuthLayout>
  )
}
