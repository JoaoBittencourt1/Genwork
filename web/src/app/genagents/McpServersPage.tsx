import { useTranslation } from 'react-i18next'
import { PageContent } from '@/shared/components/PageContent'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageSection } from '@/shared/components/PageSection'
import { GenagentsNoPermission } from './components/GenagentsNoPermission'
import { McpServersSection } from './components/McpServersSection'
import { useGenagentsAbilities } from './components/useGenagentsAbilities'

export function McpServersPage() {
  const { t } = useTranslation()
  const abilities = useGenagentsAbilities()
  return (
    <>
      <PageHeader>
        <h1 className="font-medium">{t('genagents.mcpServers.title')}</h1>
      </PageHeader>
      <PageContent>
        {abilities.canManageMcpServers ? (
          <PageSection title={t('genagents.mcpServers.title')} description={t('genagents.mcpServers.description')}>
            <McpServersSection canManage />
          </PageSection>
        ) : (
          <GenagentsNoPermission />
        )}
      </PageContent>
    </>
  )
}
