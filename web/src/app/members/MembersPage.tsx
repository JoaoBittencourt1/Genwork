import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/app/auth/components/AuthContext'
import { PERMISSIONS } from '@/app/auth/domain/session'
import { DeliveryPanel } from '@/app/invites/components/DeliveryPanel'
import { InviteForm } from '@/app/invites/components/InviteForm'
import { InviteTable } from '@/app/invites/components/InviteTable'
import type { InviteDelivery } from '@/app/invites/domain/invite'

export function MembersPage() {
  const { t } = useTranslation()
  const { can } = useAuth()
  const [lastDelivery, setLastDelivery] = useState<InviteDelivery | null>(null)

  if (!can(PERMISSIONS.inviteRead)) {
    return (
      <section className="space-y-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t('members.noPermissionTitle')}</h1>
        <p className="text-muted-foreground">{t('members.noPermissionDescription')}</p>
      </section>
    )
  }

  return (
    <section className="flex flex-col gap-6">
      <h1 className="text-2xl font-semibold tracking-tight">{t('members.title')}</h1>
      {can(PERMISSIONS.inviteCreate) ? <InviteForm onDelivered={setLastDelivery} /> : null}
      {lastDelivery ? <DeliveryPanel delivery={lastDelivery} /> : null}
      <InviteTable onDelivered={setLastDelivery} />
    </section>
  )
}
