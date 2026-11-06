import { useTranslation } from 'react-i18next'
import { PageContent } from '@/shared/components/PageContent'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageSection } from '@/shared/components/PageSection'
import { AccountsSection } from './components/AccountsSection'
import { GenagentsNoPermission } from './components/GenagentsNoPermission'
import { useGenagentsAbilities } from './components/useGenagentsAbilities'

export function AccountsPage() {
  const { t } = useTranslation()
  const abilities = useGenagentsAbilities()
  return (
    <>
      <PageHeader>
        <h1 className="font-medium">{t('genagents.accounts.title')}</h1>
      </PageHeader>
      <PageContent>
        {abilities.canManageAccounts ? (
          <PageSection title={t('genagents.accounts.title')} description={t('genagents.accounts.description')}>
            <AccountsSection canManage />
          </PageSection>
        ) : (
          <GenagentsNoPermission />
        )}
      </PageContent>
    </>
  )
}
