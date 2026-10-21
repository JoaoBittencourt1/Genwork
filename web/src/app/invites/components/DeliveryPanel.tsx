import { CopyIcon, MailCheckIcon, MailWarningIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '@/shared/ui/alert'
import { Button } from '@/shared/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import { Input } from '@/shared/ui/input'
import type { InviteDelivery } from '../domain/invite'

export function DeliveryPanel({ delivery }: { delivery: InviteDelivery }) {
  const { t } = useTranslation()

  async function copyLink() {
    await navigator.clipboard.writeText(delivery.link)
    toast.success(t('invites.copied'))
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('invites.linkTitle')}</CardTitle>
        <CardDescription>{t('invites.linkDescription')}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <div className="flex flex-col gap-2 sm:flex-row">
          <Input readOnly aria-label={t('invites.linkTitle')} value={delivery.link} className="font-mono text-xs" />
          <Button variant="outline" onClick={() => void copyLink()}>
            <CopyIcon />
            {t('invites.copy')}
          </Button>
        </div>
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
