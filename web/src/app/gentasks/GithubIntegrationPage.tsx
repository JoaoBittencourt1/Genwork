import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ExternalLinkIcon, GitPullRequestIcon, TriangleAlertIcon } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/shared/components/ConfirmDialog'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { PageContent } from '@/shared/components/PageContent'
import { PageHeader } from '@/shared/components/PageHeader'
import { useErrorMessage } from '@/shared/hooks/useErrorMessage'
import { Alert, AlertDescription, AlertTitle } from '@/shared/ui/alert'
import { Button } from '@/shared/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import { Field, FieldDescription, FieldError, FieldLabel } from '@/shared/ui/field'
import { Input } from '@/shared/ui/input'
import { Spinner } from '@/shared/ui/spinner'
import { BoardBreadcrumb } from './components/BoardBreadcrumb'
import { GentasksNoPermission } from './components/GentasksNoPermission'
import { GithubWebhookWarning } from './components/GithubWebhookWarning'
import { gentasksQueryKeys } from './components/queryKeys'
import { submitGithubManifest } from './components/submitGithubManifest'
import { useGentasksAbilities } from './components/useGentasksAbilities'
import { useGithubConnection } from './components/useGithubConnection'
import { disconnectGithub, startGithubManifest } from './datasource/githubDatasource'
import type { GithubConnection } from './domain/types'

const GITHUB_LOGIN_MAX_LENGTH = 39

export function GithubIntegrationPage() {
  const { t } = useTranslation()
  const abilities = useGentasksAbilities()
  const connection = useGithubConnection(abilities.canManageGithub)

  if (!abilities.canManageGithub) return <GentasksNoPermission />

  return (
    <>
      <PageHeader>
        <BoardBreadcrumb current={t('gentasks.github.title')} />
      </PageHeader>
      <PageContent className="max-w-3xl">
        <div className="flex flex-col gap-6">
          <p className="text-sm text-muted-foreground">{t('gentasks.github.intro')}</p>
          {connection.isPending ? <Spinner /> : null}
          {connection.isError ? <ErrorAlert error={connection.error} /> : null}
          {connection.data ? (
            <>
              <GithubWebhookWarning webhookUrl={connection.data.webhookUrl} />
              {connection.data.state === 'disconnected' ? <ConnectGithubForm /> : <ConnectedGithubApp connection={connection.data} />}
            </>
          ) : null}
        </div>
      </PageContent>
    </>
  )
}

function ConnectGithubForm() {
  const { t } = useTranslation()
  const { describeFieldError } = useErrorMessage()
  const [organization, setOrganization] = useState('')
  const start = useMutation({
    mutationFn: () => startGithubManifest(organization.trim() === '' ? null : organization.trim()),
    onSuccess: submitGithubManifest,
  })
  const organizationError = describeFieldError(start.error, 'organization')

  function submit(event: FormEvent) {
    event.preventDefault()
    start.mutate()
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('gentasks.github.connectTitle')}</CardTitle>
        <CardDescription>{t('gentasks.github.connectDescription')}</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="flex flex-col gap-5">
          {organizationError ? null : <ErrorAlert error={start.error} />}
          <Field data-invalid={Boolean(organizationError)}>
            <FieldLabel htmlFor="github-organization">{t('gentasks.github.organization')}</FieldLabel>
            <Input
              id="github-organization"
              value={organization}
              maxLength={GITHUB_LOGIN_MAX_LENGTH}
              placeholder={t('gentasks.github.organizationPlaceholder')}
              className="w-72"
              aria-invalid={Boolean(organizationError)}
              onChange={(event) => setOrganization(event.target.value)}
            />
            <FieldDescription>{t('gentasks.github.organizationDescription')}</FieldDescription>
            {organizationError ? <FieldError>{organizationError}</FieldError> : null}
          </Field>
          <div>
            {/* Stays pending on success: the page is leaving for GitHub. */}
            <Button type="submit" size="sm" disabled={start.isPending || start.isSuccess}>
              <GitPullRequestIcon />
              {start.isPending || start.isSuccess ? t('gentasks.github.redirecting') : t('gentasks.github.connect')}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}

function ConnectedGithubApp({ connection }: { connection: GithubConnection }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [confirming, setConfirming] = useState(false)
  const disconnect = useMutation({
    mutationFn: disconnectGithub,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: gentasksQueryKeys.githubConnection }),
  })
  const unreadable = connection.state === 'unreadable'

  return (
    <div className="flex flex-col gap-4">
      {unreadable ? (
        <Alert variant="destructive">
          <TriangleAlertIcon />
          <AlertTitle>{t('gentasks.github.unreadableTitle')}</AlertTitle>
          <AlertDescription>
            <p>{t('gentasks.github.unreadableDescription')}</p>
          </AlertDescription>
        </Alert>
      ) : null}
      <ErrorAlert error={disconnect.error} />
      <Card>
        <CardHeader>
          <CardTitle>{unreadable ? t('gentasks.github.appTitle') : t('gentasks.github.connectedTitle')}</CardTitle>
          <CardDescription>{t('gentasks.github.connectedDescription', { app: connection.appSlug, owner: connection.ownerLogin })}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          {!unreadable && connection.installUrl ? (
            <Button size="sm" asChild>
              <a href={connection.installUrl}>
                {t('gentasks.github.chooseRepositories')}
                <ExternalLinkIcon />
              </a>
            </Button>
          ) : null}
          {connection.appUrl ? (
            <Button size="sm" variant="outline" asChild>
              <a href={connection.appUrl} target="_blank" rel="noreferrer">
                {t('gentasks.github.appSettings')}
                <ExternalLinkIcon />
              </a>
            </Button>
          ) : null}
          <Button size="sm" variant="destructive" disabled={disconnect.isPending} onClick={() => setConfirming(true)}>
            {t('gentasks.github.disconnect')}
          </Button>
        </CardContent>
      </Card>
      <ConfirmDialog
        open={confirming}
        title={t('gentasks.github.disconnectTitle')}
        description={t('gentasks.github.disconnectDescription')}
        confirmLabel={t('gentasks.github.disconnect')}
        onConfirm={() => disconnect.mutate()}
        onOpenChange={setConfirming}
      />
    </div>
  )
}
