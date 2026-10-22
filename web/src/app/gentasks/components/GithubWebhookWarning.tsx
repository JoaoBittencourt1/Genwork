import { TriangleAlertIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/shared/ui/alert'
import { isWebhookReachableByGithub } from '../domain/webhookReachability'

export function GithubWebhookWarning({ webhookUrl }: { webhookUrl: string }) {
  const { t } = useTranslation()
  if (isWebhookReachableByGithub(webhookUrl)) return null
  return (
    <Alert>
      <TriangleAlertIcon />
      <AlertTitle>{t('gentasks.github.unreachableTitle')}</AlertTitle>
      <AlertDescription>
        <p>{t('gentasks.github.unreachableDescription', { url: webhookUrl })}</p>
      </AlertDescription>
    </Alert>
  )
}
