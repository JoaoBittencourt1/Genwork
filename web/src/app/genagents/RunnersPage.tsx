import { useTranslation } from 'react-i18next'
import { PageContent } from '@/shared/components/PageContent'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageSection } from '@/shared/components/PageSection'
import { GenagentsNoPermission } from './components/GenagentsNoPermission'
import { RunnersSection } from './components/RunnersSection'
import { useGenagentsAbilities } from './components/useGenagentsAbilities'

export function RunnersPage() {
  const { t } = useTranslation()
  const abilities = useGenagentsAbilities()
  return (
    <>
      <PageHeader>
        <h1 className="font-medium">{t('genagents.runners.title')}</h1>
      </PageHeader>
      <PageContent>
        {abilities.canManageRunners ? (
          <PageSection title={t('genagents.runners.title')} description={t('genagents.runners.description')}>
            <RunnersSection canManage />
          </PageSection>
        ) : (
          <GenagentsNoPermission />
        )}
      </PageContent>
    </>
  )
}
