import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useLocation } from '@tanstack/react-router'
import { CircleAlertIcon, CircleCheckIcon, ExternalLinkIcon } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { PageContent } from '@/shared/components/PageContent'
import { PageHeader } from '@/shared/components/PageHeader'
import { Alert, AlertDescription, AlertTitle } from '@/shared/ui/alert'
import { Button } from '@/shared/ui/button'
import { Spinner } from '@/shared/ui/spinner'
import { BoardBreadcrumb } from './components/BoardBreadcrumb'
import { GentasksNoPermission } from './components/GentasksNoPermission'
import { gentasksQueryKeys } from './components/queryKeys'
import { useGentasksAbilities } from './components/useGentasksAbilities'
import { completeGithubConnection } from './datasource/githubDatasource'
import { githubConnectionErrorKey, readManifestCallback } from './domain/githubConnection'

export function GithubCallbackPage() {
  const { t } = useTranslation()
  const abilities = useGentasksAbilities()
  const queryClient = useQueryClient()
  const callback = readManifestCallback(useLocation({ select: (location) => location.searchStr }))
  const complete = useMutation({
    mutationFn: () => completeGithubConnection(callback?.code ?? '', callback?.state ?? ''),
    onSuccess: (connection) => queryClient.setQueryData(gentasksQueryKeys.githubConnection, connection),
  })
  // GitHub's code works once; the ref keeps React's development double effect from spending it twice.
  const started = useRef(false)

  useEffect(() => {
    if (started.current || !callback || !abilities.canManageGithub) return
    started.current = true
    complete.mutate()
  }, [callback, abilities.canManageGithub, complete])

  if (!abilities.canManageGithub) return <GentasksNoPermission />

  const failureKey = !callback ? 'gentasks.github.errors.callbackMissing' : complete.isError ? githubConnectionErrorKey(complete.error) : null

  return (
    <>
      <PageHeader>
        <BoardBreadcrumb current={t('gentasks.github.title')} />
      </PageHeader>
      <PageContent className="max-w-3xl">
        <div className="flex flex-col gap-4">
          {failureKey ? (
            <>
              <Alert variant="destructive">
                <CircleAlertIcon />
                <AlertTitle>{t('gentasks.github.connectFailed')}</AlertTitle>
                <AlertDescription>
                  <p>{t(failureKey)}</p>
                </AlertDescription>
              </Alert>
              <div>
                <Button size="sm" variant="outline" asChild>
                  <Link to="/tasks/integrations/github">{t('gentasks.github.startAgain')}</Link>
                </Button>
              </div>
            </>
          ) : null}
          {!failureKey && !complete.isSuccess ? (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Spinner />
              {t('gentasks.github.finishing')}
            </div>
          ) : null}
          {complete.data ? (
            <>
              <Alert>
                <CircleCheckIcon />
                <AlertTitle>{t('gentasks.github.connectedTitle')}</AlertTitle>
                <AlertDescription>
                  <p>{t('gentasks.github.installNext', { app: complete.data.appSlug })}</p>
                </AlertDescription>
              </Alert>
              <div className="flex flex-wrap gap-2">
                {complete.data.installUrl ? (
                  <Button size="sm" asChild>
                    <a href={complete.data.installUrl}>
                      {t('gentasks.github.chooseRepositories')}
                      <ExternalLinkIcon />
                    </a>
                  </Button>
                ) : null}
                <Button size="sm" variant="outline" asChild>
                  <Link to="/tasks/integrations/github">{t('gentasks.github.backToIntegration')}</Link>
                </Button>
              </div>
            </>
          ) : null}
        </div>
      </PageContent>
    </>
  )
}
