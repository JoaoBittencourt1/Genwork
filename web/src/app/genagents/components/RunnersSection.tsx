import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { HardDriveIcon, PlusIcon, TriangleAlertIcon } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { CopyableValue } from '@/shared/components/CopyableValue'
import { ConfirmDialog } from '@/shared/components/ConfirmDialog'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { Badge } from '@/shared/ui/badge'
import { Button } from '@/shared/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/shared/ui/dialog'
import { Field, FieldLabel } from '@/shared/ui/field'
import { Input } from '@/shared/ui/input'
import { Spinner } from '@/shared/ui/spinner'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/ui/table'
import { cleanRunners, createRunner, listRunners, removeRunner, revokeRunner } from '../datasource/genagentsDatasource'
import { formatBytes } from '@/shared/domain/byteUnits'
import { runnerCanBeRemoved, runnerCleanupState, runnerRegisterCommand, runnerStatus, runnerWarnings } from '../domain/runners'
import type { CreatedRunner, Runner } from '../domain/types'
import { genagentsQueryKeys } from './queryKeys'

export function RunnersSection({ canManage }: { canManage: boolean }) {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const runners = useQuery({ queryKey: genagentsQueryKeys.runners, queryFn: listRunners, enabled: canManage, refetchInterval: 10_000 })
  const [creating, setCreating] = useState(false)
  const [created, setCreated] = useState<CreatedRunner | null>(null)
  const [revoking, setRevoking] = useState<Runner | null>(null)
  const [removing, setRemoving] = useState<Runner | null>(null)
  const refresh = () => queryClient.invalidateQueries({ queryKey: genagentsQueryKeys.runners })
  const revoke = useMutation({ mutationFn: (runner: Runner) => revokeRunner(runner.token), onSuccess: refresh })
  const remove = useMutation({ mutationFn: (runner: Runner) => removeRunner(runner.token), onSuccess: refresh })
  const clean = useMutation({ mutationFn: cleanRunners, onSuccess: refresh })

  if (runners.isPending) return <Spinner />
  if (runners.isLoadingError) return <ErrorAlert error={runners.error} />

  return (
    <div className="flex flex-col gap-3">
      <ErrorAlert error={revoke.error ?? remove.error ?? clean.error} />
      {clean.data ? <p className="text-sm text-muted-foreground">{t('genagents.runners.cleanup.asked', { count: clean.data.runnersAsked })}</p> : null}
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('genagents.runners.columns.name')}</TableHead>
              <TableHead>{t('genagents.runners.columns.status')}</TableHead>
              <TableHead>{t('genagents.runners.columns.sandbox')}</TableHead>
              <TableHead>{t('genagents.runners.columns.tools')}</TableHead>
              <TableHead className="text-right">
                <span className="sr-only">{t('genagents.actions')}</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {runners.data.map((runner) => {
              const removable = runnerCanBeRemoved(runner, new Date())
              return (
                <TableRow key={runner.token}>
                  <TableCell className="font-medium">{runner.name}</TableCell>
                  <TableCell>
                    <div className="flex flex-col gap-1">
                      <Badge variant={runnerStatus(runner) === 'online' ? 'secondary' : 'outline'}>{t(`genagents.runners.statuses.${runnerStatus(runner)}`)}</Badge>
                      {runnerWarnings(runner).map((warning) => (
                        <span key={warning} className="flex items-center gap-1 text-xs text-amber-600 dark:text-amber-400">
                          <TriangleAlertIcon className="size-3" aria-hidden />
                          {t(`genagents.runners.warnings.${warning}`)}
                        </span>
                      ))}
                    </div>
                  </TableCell>
                  <TableCell className="text-xs">
                    {runner.directoryMode ? t('genagents.runners.directoryMode') : (runner.containerEngine ?? '—')}
                    {runner.sandboxImage ? <div className="font-mono text-muted-foreground">{runner.sandboxImage}</div> : null}
                    <RunnerCleanupNote runner={runner} formatSize={(size) => formatBytes(size, i18n.language)} />
                  </TableCell>
                  <TableCell className="text-xs">{Object.entries(runner.tools).map(([cli, version]) => `${cli} ${version}`).join(', ') || '—'}</TableCell>
                  <TableCell className="text-right">
                    {canManage && removable ? (
                      <Button size="sm" variant="ghost" onClick={() => setRemoving(runner)}>
                        {t('common.delete')}
                      </Button>
                    ) : null}
                    {canManage && !removable ? (
                      <Button size="sm" variant="ghost" onClick={() => setRevoking(runner)}>
                        {t('genagents.runners.revoke')}
                      </Button>
                    ) : null}
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </div>
      {canManage ? (
        <div className="flex gap-2">
          <Button onClick={() => setCreating(true)}>
            <PlusIcon />
            {t('genagents.runners.add')}
          </Button>
          <Button variant="outline" onClick={() => clean.mutate()} disabled={clean.isPending}>
            <HardDriveIcon />
            {t('genagents.runners.cleanup.action')}
          </Button>
        </div>
      ) : null}
      <Dialog open={creating} onOpenChange={setCreating}>
        <DialogContent>{creating ? <RunnerForm onDone={() => setCreating(false)} onCreated={(runner) => { setCreated(runner); void refresh() }} /> : null}</DialogContent>
      </Dialog>
      <Dialog open={created !== null} onOpenChange={(open) => !open && setCreated(null)}>
        <DialogContent className="min-w-0 overflow-hidden sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{t('genagents.runners.registerTitle')}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">{t('genagents.runners.registerDescription')}</p>
          {created ? <CopyableValue multiline label={t('genagents.runners.registerCommand')} value={runnerRegisterCommand(window.location.origin, created.registrationToken)} /> : null}
        </DialogContent>
      </Dialog>
      <ConfirmDialog
        open={revoking !== null}
        title={t('genagents.runners.revokeTitle')}
        description={t('genagents.runners.revokeDescription', { name: revoking?.name ?? '' })}
        confirmLabel={t('genagents.runners.revoke')}
        onOpenChange={(open) => !open && setRevoking(null)}
        onConfirm={() => {
          if (revoking) revoke.mutate(revoking)
          setRevoking(null)
        }}
      />
      <ConfirmDialog
        open={removing !== null}
        title={t('genagents.runners.deleteTitle')}
        description={t('genagents.runners.deleteDescription', { name: removing?.name ?? '' })}
        confirmLabel={t('common.delete')}
        onOpenChange={(open) => !open && setRemoving(null)}
        onConfirm={() => {
          if (removing) remove.mutate(removing)
          setRemoving(null)
        }}
      />
    </div>
  )
}

function RunnerForm({ onDone, onCreated }: { onDone: () => void; onCreated: (runner: CreatedRunner) => void }) {
  const { t } = useTranslation()
  const [name, setName] = useState('')
  const create = useMutation({
    mutationFn: () => createRunner(name.trim()),
    onSuccess: (runner) => {
      onCreated(runner)
      onDone()
    },
  })

  function submit(event: FormEvent) {
    event.preventDefault()
    create.mutate()
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      <DialogHeader>
        <DialogTitle>{t('genagents.runners.add')}</DialogTitle>
      </DialogHeader>
      <ErrorAlert error={create.error} />
      <Field>
        <FieldLabel htmlFor="runner-name">{t('genagents.runners.columns.name')}</FieldLabel>
        <Input id="runner-name" value={name} required autoFocus onChange={(event) => setName(event.target.value)} />
      </Field>
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onDone}>
          {t('common.cancel')}
        </Button>
        <Button type="submit" disabled={create.isPending || name.trim() === ''}>
          {t('genagents.runners.create')}
        </Button>
      </DialogFooter>
    </form>
  )
}

function RunnerCleanupNote({ runner, formatSize }: { runner: Runner; formatSize: (size: number) => string }) {
  const { t, i18n } = useTranslation()
  const state = runnerCleanupState(runner)
  if (state.kind === 'none') return null
  if (state.kind === 'pending') return <div className="text-muted-foreground">{t('genagents.runners.cleanup.pending')}</div>
  return (
    <div className="text-muted-foreground">
      {t('genagents.runners.cleanup.done', { size: formatSize(state.freedBytes), time: new Date(state.at).toLocaleString(i18n.language) })}
    </div>
  )
}
