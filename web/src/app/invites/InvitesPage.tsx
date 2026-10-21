import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/app/auth/components/AuthContext'
import { PERMISSIONS } from '@/app/auth/domain/session'
import { DeliveryPanel } from './components/DeliveryPanel'
import { InviteForm } from './components/InviteForm'
import { InviteTable } from './components/InviteTable'
import type { InviteDelivery } from './domain/invite'

export function InvitesPage() {
  const { t } = useTranslation()
  const { can } = useAuth()
  const [lastDelivery, setLastDelivery] = useState<InviteDelivery | null>(null)

  if (!can(PERMISSIONS.inviteRead)) {
    return (
      <section className="space-y-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t('invites.noPermissionTitle')}</h1>
        <p className="text-muted-foreground">{t('invites.noPermissionDescription')}</p>
      </section>
    )
  }

  return (
    <section className="flex flex-col gap-6">
      <h1 className="text-2xl font-semibold tracking-tight">{t('invites.title')}</h1>
      {can(PERMISSIONS.inviteCreate) ? <InviteForm onDelivered={setLastDelivery} /> : null}
      {lastDelivery ? <DeliveryPanel delivery={lastDelivery} /> : null}
      <InviteTable onDelivered={setLastDelivery} />
    </section>
  )
}
