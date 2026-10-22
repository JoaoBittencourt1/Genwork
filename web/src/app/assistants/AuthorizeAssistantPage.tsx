import { useLocation } from '@tanstack/react-router'
import { CircleAlertIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/app/auth/components/AuthContext'
import { AuthLayout } from '@/shared/components/AuthLayout'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { LoadingScreen } from '@/shared/components/LoadingScreen'
import { Alert, AlertDescription } from '@/shared/ui/alert'
import { Button } from '@/shared/ui/button'
import { Spinner } from '@/shared/ui/spinner'
import { useAuthorizationPreview } from './components/useAuthorizationPreview'
import { parseAuthorizationRequest } from './domain/authorizationRequest'

export function AuthorizeAssistantPage() {
  const { t } = useTranslation()
  const { state } = useAuth()
  const searchStr = useLocation({ select: (location) => location.searchStr })
  const request = parseAuthorizationRequest(searchStr)
  const { preview, decision } = useAuthorizationPreview(request)

  if (request === null || preview.isError) {
    return (
      <AuthLayout title={t('assistants.consent.invalidTitle')}>
        <Alert variant="destructive">
          <CircleAlertIcon />
          <AlertDescription>{t('assistants.consent.invalid')}</AlertDescription>
        </Alert>
      </AuthLayout>
    )
  }
  if (!preview.data || preview.data.kind === 'redirect' || state.status !== 'signedIn') return <LoadingScreen />

  const { clientName } = preview.data
  const leaving = decision.isPending || decision.isSuccess
  return (
    <AuthLayout title={t('assistants.consent.title', { client: clientName })} description={t('assistants.consent.signedInAs', { email: state.me.email })}>
      <div className="flex flex-col gap-4">
        <p className="text-sm">{t('assistants.consent.actsAsYou', { client: clientName })}</p>
        <p className="text-sm text-muted-foreground">{t('assistants.consent.noPassword')}</p>
        <ErrorAlert error={decision.error} />
        <div className="flex gap-2">
          <Button variant="outline" className="flex-1" disabled={leaving} onClick={() => decision.mutate('deny')}>
            {t('assistants.consent.deny')}
          </Button>
          <Button className="flex-1" disabled={leaving} onClick={() => decision.mutate('allow')}>
            {leaving ? <Spinner /> : null}
            {t('assistants.consent.allow')}
          </Button>
        </div>
      </div>
    </AuthLayout>
  )
}
