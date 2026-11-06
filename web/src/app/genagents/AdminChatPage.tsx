import { useTranslation } from 'react-i18next'
import { PageContent } from '@/shared/components/PageContent'
import { PageHeader } from '@/shared/components/PageHeader'
import { AdminChatPanel } from './components/AdminChatPanel'
import { GenagentsNoPermission } from './components/GenagentsNoPermission'
import { useGenagentsAbilities } from './components/useGenagentsAbilities'

export function AdminChatPage() {
  const { t } = useTranslation()
  const abilities = useGenagentsAbilities()
  return (
    <>
      <PageHeader>
        <h1 className="font-medium">{t('genagents.adminChat.title')}</h1>
      </PageHeader>
      <PageContent className="max-w-6xl">{abilities.canUseAdminChat ? <AdminChatPanel /> : <GenagentsNoPermission />}</PageContent>
    </>
  )
}
