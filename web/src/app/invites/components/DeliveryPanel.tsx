import { MailCheckIcon, MailWarningIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { CopyableValue } from '@/shared/components/CopyableValue'
import { Alert, AlertDescription } from '@/shared/ui/alert'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import type { InviteDelivery } from '../domain/invite'

export function DeliveryPanel({ delivery }: { delivery: InviteDelivery }) {
  const { t } = useTranslation()

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('invites.linkTitle')}</CardTitle>
        <CardDescription>{t('invites.linkDescription')}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <CopyableValue value={delivery.link} label={t('invites.linkTitle')} copyLabel={t('invites.copy')} copiedMessage={t('invites.copied')} />
        {delivery.emailSent ? (
          <Alert>
            <MailCheckIcon />
            <AlertDescription>{t('invites.emailSent', { email: delivery.invite.email })}</AlertDescription>
          </Alert>
        ) : (
          <Alert variant="destructive">
            <MailWarningIcon />
            <AlertDescription>{t('invites.emailNotSent')}</AlertDescription>
          </Alert>
        )}
      </CardContent>
    </Card>
  )
}
