import { Link, Navigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/app/auth/components/AuthContext'
import { AuthLayout } from '@/shared/components/AuthLayout'
import { LoadingScreen } from '@/shared/components/LoadingScreen'
import { NO_LIVE_REFRESH } from '@/shared/domain/liveRefresh'
import { Button } from '@/shared/ui/button'
import { AcceptInviteForm } from './components/AcceptInviteForm'
import { inspectInvitation } from './datasource/invitesDatasource'

export function AcceptInvitePage({ secret }: { secret: string }) {
  const { t } = useTranslation()
  const { state } = useAuth()
  const invitation = useQuery({
    queryKey: ['invitation', secret],
    queryFn: () => inspectInvitation(secret),
    retry: false,
    ...NO_LIVE_REFRESH,
  })

  if (state.status === 'signedIn') return <Navigate to="/" />
  if (invitation.isPending) return <LoadingScreen />
  if (invitation.isError) {
    return (
      <AuthLayout title={t('acceptInvite.notFoundTitle')} description={t('acceptInvite.notFoundDescription')}>
        <Button asChild variant="outline" className="w-full">
          <Link to="/login">{t('acceptInvite.goToLogin')}</Link>
        </Button>
      </AuthLayout>
    )
  }
  return (
    <AuthLayout
      title={t('acceptInvite.title')}
      description={t('acceptInvite.subtitle', { instance: invitation.data.instanceName })}
    >
      <AcceptInviteForm secret={secret} email={invitation.data.email} />
    </AuthLayout>
  )
}
