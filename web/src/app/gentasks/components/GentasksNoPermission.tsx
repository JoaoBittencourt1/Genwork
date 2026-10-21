import { useTranslation } from 'react-i18next'
import { PageContent } from '@/shared/components/PageContent'
import { PageHeader } from '@/shared/components/PageHeader'
import { BoardBreadcrumb } from './BoardBreadcrumb'

export function GentasksNoPermission() {
  const { t } = useTranslation()
  return (
    <>
      <PageHeader>
        <BoardBreadcrumb />
      </PageHeader>
      <PageContent>
        <section className="space-y-2">
          <h2 className="text-2xl font-semibold tracking-tight">{t('gentasks.noPermissionTitle')}</h2>
          <p className="text-muted-foreground">{t('gentasks.noPermissionDescription')}</p>
        </section>
      </PageContent>
    </>
  )
}
