import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/app/auth/components/AuthContext'
import { PERMISSIONS } from '@/app/auth/domain/session'
import { DeliveryPanel } from '@/app/invites/components/DeliveryPanel'
import { InviteForm } from '@/app/invites/components/InviteForm'
import { InviteTable } from '@/app/invites/components/InviteTable'
import type { InviteDelivery } from '@/app/invites/domain/invite'
import { PageContent } from '@/shared/components/PageContent'
import { PageHeader } from '@/shared/components/PageHeader'

export function MembersPage() {
  const { t } = useTranslation()
  const { can } = useAuth()
  const [lastDelivery, setLastDelivery] = useState<InviteDelivery | null>(null)

  return (
    <>
      <PageHeader>
        <h1 className="font-medium">{t('members.title')}</h1>
      </PageHeader>
      <PageContent>
        {can(PERMISSIONS.inviteRead) ? (
          <section className="flex flex-col gap-6">
            {can(PERMISSIONS.inviteCreate) ? <InviteForm onDelivered={setLastDelivery} /> : null}
            {lastDelivery ? <DeliveryPanel delivery={lastDelivery} /> : null}
            <InviteTable onDelivered={setLastDelivery} />
          </section>
        ) : (
          <section className="space-y-2">
            <h2 className="text-2xl font-semibold tracking-tight">{t('members.noPermissionTitle')}</h2>
            <p className="text-muted-foreground">{t('members.noPermissionDescription')}</p>
          </section>
        )}
      </PageContent>
    </>
  )
}
