import { TriangleAlertIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { CopyableValue } from '@/shared/components/CopyableValue'
import { PageContent } from '@/shared/components/PageContent'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageSection } from '@/shared/components/PageSection'
import { Alert, AlertDescription, AlertTitle } from '@/shared/ui/alert'
import { buildClaudeCodeCommand, buildClientConfigJson, buildMcpUrl, needsReachabilityWarning } from './domain/connection'

export function ConnectAssistantPage() {
  const { t } = useTranslation()
  const origin = window.location.origin
  const mcpUrl = buildMcpUrl(origin)

  return (
    <>
      <PageHeader>
        <h1 className="font-medium">{t('app.nav.assistants')}</h1>
      </PageHeader>
      <PageContent className="max-w-3xl space-y-8">
        <section className="space-y-2">
          <h2 className="text-2xl font-semibold tracking-tight">{t('assistants.connect.title')}</h2>
          <p className="text-muted-foreground">{t('assistants.connect.description')}</p>
        </section>
        <PageSection title={t('assistants.connect.url.title')} description={t('assistants.connect.url.description')}>
          <CopyableValue value={mcpUrl} label={t('assistants.connect.url.title')} copiedMessage={t('assistants.connect.url.copied')} />
        </PageSection>
        <PageSection title={t('assistants.connect.claude.title')} description={t('assistants.connect.claude.description')}>
          {needsReachabilityWarning(origin) ? (
            <Alert>
              <TriangleAlertIcon />
              <AlertTitle>{t('assistants.connect.claude.unreachableTitle')}</AlertTitle>
              <AlertDescription>{t('assistants.connect.claude.unreachable')}</AlertDescription>
            </Alert>
          ) : null}
          <ol className="list-decimal space-y-2 pl-5 text-sm">
            <li>{t('assistants.connect.claude.steps.open')}</li>
            <li>{t('assistants.connect.claude.steps.add')}</li>
            <li>{t('assistants.connect.claude.steps.signIn')}</li>
          </ol>
        </PageSection>
        <PageSection title={t('assistants.connect.claudeCode.title')} description={t('assistants.connect.claudeCode.description')}>
          <CopyableValue value={buildClaudeCodeCommand(mcpUrl)} label={t('assistants.connect.claudeCode.title')} />
          <p className="text-sm text-muted-foreground">{t('assistants.connect.claudeCode.signIn')}</p>
        </PageSection>
        <PageSection title={t('assistants.connect.other.title')} description={t('assistants.connect.other.description')}>
          <CopyableValue value={buildClientConfigJson(mcpUrl)} label={t('assistants.connect.other.title')} multiline />
        </PageSection>
      </PageContent>
    </>
  )
}
