import { useTranslation } from 'react-i18next'
import { PageContent } from '@/shared/components/PageContent'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageSection } from '@/shared/components/PageSection'
import { GenagentsNoPermission } from './components/GenagentsNoPermission'
import { SkillsSection } from './components/SkillsSection'
import { useGenagentsAbilities } from './components/useGenagentsAbilities'

export function SkillsPage() {
  const { t } = useTranslation()
  const abilities = useGenagentsAbilities()
  return (
    <>
      <PageHeader>
        <h1 className="font-medium">{t('genagents.skills.title')}</h1>
      </PageHeader>
      <PageContent>
        {abilities.canManageSkills ? (
          <PageSection title={t('genagents.skills.title')} description={t('genagents.skills.description')}>
            <SkillsSection canManage />
          </PageSection>
        ) : (
          <GenagentsNoPermission />
        )}
      </PageContent>
    </>
  )
}
