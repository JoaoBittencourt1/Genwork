import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { Button } from '@/shared/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/shared/ui/dialog'
import { Spinner } from '@/shared/ui/spinner'
import { loadError } from '@/shared/domain/errors'
import { deleteTask, listArchivedTasks, unarchiveTask } from '../datasource/tasksDatasource'
import type { GentasksAbilities } from '../domain/permissions'
import { taskIdentifier } from '../domain/taskPrefix'
import type { Task } from '../domain/types'
import { gentasksQueryKeys } from './queryKeys'

type ArchivedTasksDialogProps = {
  boardToken: string
  taskPrefix: string
  abilities: GentasksAbilities
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function ArchivedTasksDialog({ boardToken, taskPrefix, abilities, open, onOpenChange }: ArchivedTasksDialogProps) {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const archived = useQuery({ queryKey: gentasksQueryKeys.archived(boardToken), queryFn: () => listArchivedTasks(boardToken), enabled: open })
  const formatDate = new Intl.DateTimeFormat(i18n.language, { dateStyle: 'short', timeStyle: 'short' })
  const refresh = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: gentasksQueryKeys.archived(boardToken) }),
      queryClient.invalidateQueries({ queryKey: gentasksQueryKeys.board(boardToken) }),
    ])
  const restore = useMutation({ mutationFn: (task: Task) => unarchiveTask(task.token), onSuccess: refresh })
  const remove = useMutation({ mutationFn: (task: Task) => deleteTask(task.token), onSuccess: refresh })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('gentasks.archived.title')}</DialogTitle>
        </DialogHeader>
        <ErrorAlert error={loadError(archived) ?? restore.error ?? remove.error} />
        {archived.isPending ? <Spinner /> : null}
        {archived.data?.length === 0 ? <p className="text-sm text-muted-foreground">{t('gentasks.archived.empty')}</p> : null}
        <ul className="flex max-h-96 flex-col gap-2 overflow-y-auto">
          {archived.data?.map((task) => (
            <li key={task.token} className="flex items-center gap-2 rounded-lg border p-2">
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium">
                  <span className="mr-2 text-muted-foreground tabular-nums">{taskIdentifier(taskPrefix, task.number)}</span>
                  {task.title}
                </p>
                {task.archivedAt ? (
                  <p className="text-xs text-muted-foreground">
                    {t('gentasks.archived.archivedAt', { date: formatDate.format(new Date(task.archivedAt)) })}
                  </p>
                ) : null}
              </div>
              {abilities.canWriteTasks ? (
                <Button size="sm" variant="outline" disabled={restore.isPending} onClick={() => restore.mutate(task)}>
                  {t('gentasks.archived.restore')}
                </Button>
              ) : null}
              {abilities.canDeleteTasks ? (
                <Button size="sm" variant="destructive" disabled={remove.isPending} onClick={() => remove.mutate(task)}>
                  {t('common.delete')}
                </Button>
              ) : null}
            </li>
          ))}
        </ul>
      </DialogContent>
    </Dialog>
  )
}
