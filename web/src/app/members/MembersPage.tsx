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
import { PageSection } from '@/shared/components/PageSection'
import { MemberTable } from './components/MemberTable'

export function MembersPage() {
  const { t } = useTranslation()
  const { state, can } = useAuth()
  const [lastDelivery, setLastDelivery] = useState<InviteDelivery | null>(null)
  const canReadMembers = can(PERMISSIONS.memberRead)
  const canReadInvites = can(PERMISSIONS.inviteRead)

  return (
    <>
      <PageHeader>
        <h1 className="font-medium">{t('members.title')}</h1>
      </PageHeader>
      <PageContent>
        {state.status === 'signedIn' && (canReadMembers || canReadInvites) ? (
          <div className="flex flex-col gap-10">
            {canReadMembers ? (
              <PageSection title={t('members.sectionTitle')} description={t('members.sectionDescription')}>
                <MemberTable
                  viewer={{ token: state.me.token, rolePermissions: state.me.role.permissions, canManage: can(PERMISSIONS.memberManage) }}
                />
              </PageSection>
            ) : null}
            {canReadInvites ? (
              <PageSection title={t('invites.sectionTitle')} description={t('invites.sectionDescription')}>
                {can(PERMISSIONS.inviteCreate) ? <InviteForm onDelivered={setLastDelivery} /> : null}
                {lastDelivery ? <DeliveryPanel delivery={lastDelivery} /> : null}
                <InviteTable onDelivered={setLastDelivery} />
              </PageSection>
            ) : null}
          </div>
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
