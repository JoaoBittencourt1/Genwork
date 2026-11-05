import { useTranslation } from 'react-i18next'
import { PageContent } from '@/shared/components/PageContent'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageSection } from '@/shared/components/PageSection'
import { useRoles } from '@/shared/hooks/useRoles'
import { BotsSection } from './components/BotsSection'
import { GenagentsNoPermission } from './components/GenagentsNoPermission'
import { useGenagentsAbilities } from './components/useGenagentsAbilities'

export function BotsPage() {
  const { t } = useTranslation()
  const abilities = useGenagentsAbilities()
  const roles = useRoles(abilities.canManageBots)
  return (
    <>
      <PageHeader>
        <h1 className="font-medium">{t('genagents.bots.title')}</h1>
      </PageHeader>
      <PageContent>
        {abilities.canReadBots ? (
          <PageSection title={t('genagents.bots.title')} description={t('genagents.bots.description')}>
            <BotsSection canManage={abilities.canManageBots} roles={roles.data ?? []} />
          </PageSection>
        ) : (
          <GenagentsNoPermission />
        )}
      </PageContent>
    </>
  )
}
