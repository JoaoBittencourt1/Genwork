import { CircleAlertIcon, DownloadIcon, ExternalLinkIcon, RefreshCwIcon, RotateCcwIcon } from 'lucide-react'
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/shared/components/ConfirmDialog'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { Alert, AlertDescription, AlertTitle } from '@/shared/ui/alert'
import { Button } from '@/shared/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import { Spinner } from '@/shared/ui/spinner'
import { canCheckForUpdates, type UpdateCardView } from '../domain/updateCard'
import { useUpdateStatus } from './useUpdateStatus'

export function UpdateCard() {
  const { t, i18n } = useTranslation()
  const update = useUpdateStatus(true)
  const { view } = update

  useEffect(() => {
    if (view.shouldReloadApp) window.location.reload()
  }, [view.shouldReloadApp])

  const checkedAt = view.checkedAt ? new Date(view.checkedAt).toLocaleString(i18n.language) : null
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('system.update.title')}</CardTitle>
        <CardDescription>
          {view.installedVersion ? t('system.update.installed', { version: view.installedVersion }) : t('app.loading')}
          {checkedAt ? ` · ${t('system.update.checkedAt', { time: checkedAt })}` : null}
        </CardDescription>
        {canCheckForUpdates(view) ? (
          <CardAction>
            <Button variant="outline" size="sm" disabled={view.kind === 'checking'} onClick={update.checkForUpdates}>
              {view.kind === 'checking' ? <Spinner /> : <RefreshCwIcon />}
              {view.kind === 'checking' ? t('system.update.checking') : t('system.update.check')}
            </Button>
          </CardAction>
        ) : null}
      </CardHeader>
      <CardContent className="space-y-4">
        <ErrorAlert error={update.checkError ?? update.startError} />
        <UpdateCardBody view={view} onUpdate={update.askToConfirm} />
      </CardContent>
      {view.kind === 'confirming' ? (
        <ConfirmDialog
          open
          title={t('system.update.confirm.title', { version: view.latest.version })}
          description={t('system.update.confirm.description')}
          confirmLabel={t('system.update.confirm.action', { version: view.latest.version })}
          onConfirm={() => update.confirmUpdate(view.latest.version)}
          onOpenChange={(open) => (open ? undefined : update.cancelConfirmation())}
        />
      ) : null}
    </Card>
  )
}

function UpdateCardBody({ view, onUpdate }: { view: UpdateCardView; onUpdate: () => void }) {
  const { t } = useTranslation()
  switch (view.kind) {
    case 'loading':
      return <Spinner />
    case 'idle':
    case 'checking':
      return <p className="text-sm text-muted-foreground">{t('system.update.neverChecked')}</p>
    case 'upToDate':
      return <p className="text-sm text-muted-foreground">{t('system.update.upToDate')}</p>
    case 'available':
    case 'confirming':
      return (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="space-y-1">
            <p className="text-sm font-medium">{t('system.update.available', { version: view.latest.version })}</p>
            <a href={view.latest.notesUrl} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-sm text-primary hover:underline">
              {t('system.update.releaseNotes')}
              <ExternalLinkIcon className="size-3" aria-hidden />
            </a>
          </div>
          <Button onClick={onUpdate}>
            <DownloadIcon />
            {t('system.update.updateTo', { version: view.latest.version })}
          </Button>
        </div>
      )
    case 'updating':
      return (
        <p className="flex items-center gap-2 text-sm" role="status">
          <Spinner />
          {t('system.update.updating', { version: view.targetVersion })}
          {view.step ? ` — ${t(`system.update.steps.${view.step}`)}` : null}
        </p>
      )
    case 'restarting':
      return (
        <p className="flex items-center gap-2 text-sm" role="status">
          <Spinner />
          {t('system.update.restarting')}
        </p>
      )
    case 'succeeded':
      return (
        <p className="flex items-center gap-2 text-sm" role="status">
          <Spinner />
          {t('system.update.succeeded')}
        </p>
      )
    case 'failed':
      return (
        <Alert variant="destructive">
          {view.failure.rolledBack ? <RotateCcwIcon /> : <CircleAlertIcon />}
          <AlertTitle>{t('system.update.failed', { version: view.targetVersion, step: t(`system.update.steps.${view.failure.step}`) })}</AlertTitle>
          <AlertDescription>
            {t(`errors.${view.failure.code}`, { defaultValue: t('errors.unknown_error') })}{' '}
            {view.failure.rolledBack ? t('system.update.rolledBack') : t('system.update.notRolledBack')}
          </AlertDescription>
        </Alert>
      )
    case 'gaveUp':
      return (
        <Alert variant="destructive">
          <CircleAlertIcon />
          <AlertDescription>{t('system.update.gaveUp')}</AlertDescription>
        </Alert>
      )
    case 'notUpdatable':
      return (
        <p className="text-sm text-muted-foreground">
          {t(`system.update.notUpdatable.${view.reason}`, { defaultValue: t('system.update.notUpdatable.unknown') })}
        </p>
      )
  }
}
