import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ArchiveIcon, ArrowLeftIcon, PlusIcon, TagIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/shared/components/ConfirmDialog'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { useErrorMessage } from '@/shared/hooks/useErrorMessage'
import { Button } from '@/shared/ui/button'
import { Spinner } from '@/shared/ui/spinner'
import { ArchivedTasksDialog } from './components/ArchivedTasksDialog'
import { BoardCanvas } from './components/BoardCanvas'
import { BoardFilters } from './components/BoardFilters'
import { LabelManagerDialog } from './components/LabelManagerDialog'
import { NameDialog } from './components/NameDialog'
import { gentasksQueryKeys } from './components/queryKeys'
import { TaskDialog, type TaskDialogMode } from './components/TaskDialog'
import { useDirectoryUsers } from './components/useDirectoryUsers'
import { useGentasksAbilities } from './components/useGentasksAbilities'
import { addColumn, deleteColumn, readBoard, renameColumn } from './datasource/boardsDatasource'
import { resolveColumnTitle } from './domain/columnName'
import { isTaskDragEnabled } from './domain/dragAndDrop'
import { EMPTY_FILTER, filterBoardView, isFilterActive, type TaskFilter } from './domain/filters'
import type { Column } from './domain/types'

type ColumnNameEditing = { kind: 'add' } | { kind: 'rename'; column: Column }

export function BoardPage({ boardToken }: { boardToken: string }) {
  const { t } = useTranslation()
  const { describeError } = useErrorMessage()
  const abilities = useGentasksAbilities()
  const queryClient = useQueryClient()
  const board = useQuery({ queryKey: gentasksQueryKeys.board(boardToken), queryFn: () => readBoard(boardToken), enabled: abilities.canRead })
  const users = useDirectoryUsers()
  const [filter, setFilter] = useState<TaskFilter>(EMPTY_FILTER)
  const [taskDialog, setTaskDialog] = useState<TaskDialogMode | null>(null)
  const [columnEditing, setColumnEditing] = useState<ColumnNameEditing | null>(null)
  const [columnDeleting, setColumnDeleting] = useState<Column | null>(null)
  const [labelsOpen, setLabelsOpen] = useState(false)
  const [archivedOpen, setArchivedOpen] = useState(false)
  const refresh = () => queryClient.invalidateQueries({ queryKey: gentasksQueryKeys.board(boardToken) })

  const saveColumnName = useMutation({
    mutationFn: (name: string) =>
      columnEditing?.kind === 'rename' ? renameColumn(columnEditing.column.token, name) : addColumn(boardToken, name),
    onSuccess: async () => {
      setColumnEditing(null)
      await refresh()
    },
  })
  const resetColumnName = useMutation({
    mutationFn: (column: Column) => renameColumn(column.token, null),
    onSuccess: refresh,
    onError: (error) => toast.error(describeError(error)),
  })
  const removeColumn = useMutation({
    mutationFn: (column: Column) => deleteColumn(column.token),
    onSuccess: refresh,
    onError: (error) => toast.error(describeError(error)),
  })

  if (!abilities.canRead) {
    return (
      <section className="space-y-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t('gentasks.noPermissionTitle')}</h1>
        <p className="text-muted-foreground">{t('gentasks.noPermissionDescription')}</p>
      </section>
    )
  }
  if (board.isPending) return <Spinner />
  if (board.isError) return <ErrorAlert error={board.error} />

  function openColumnEditing(next: ColumnNameEditing) {
    saveColumnName.reset()
    setColumnEditing(next)
  }

  const taskDragEnabled = isTaskDragEnabled(abilities.canWriteTasks, filter)

  return (
    <section className="flex h-[calc(100svh-8rem)] flex-col gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <Button variant="ghost" size="icon-sm" asChild>
          <Link to="/tasks" aria-label={t('gentasks.board.back')}>
            <ArrowLeftIcon />
          </Link>
        </Button>
        <h1 className="text-2xl font-semibold tracking-tight">{board.data.name}</h1>
        <div className="ml-auto flex flex-wrap gap-2">
          {abilities.canManageBoards ? (
            <Button variant="outline" size="sm" onClick={() => setLabelsOpen(true)}>
              <TagIcon />
              {t('gentasks.board.labels')}
            </Button>
          ) : null}
          <Button variant="outline" size="sm" onClick={() => setArchivedOpen(true)}>
            <ArchiveIcon />
            {t('gentasks.board.archived')}
          </Button>
          {abilities.canManageBoards ? (
            <Button size="sm" onClick={() => openColumnEditing({ kind: 'add' })}>
              <PlusIcon />
              {t('gentasks.board.addColumn')}
            </Button>
          ) : null}
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <BoardFilters filter={filter} labels={board.data.labels} users={users.data} onChange={setFilter} />
        {abilities.canWriteTasks && isFilterActive(filter) ? (
          <p className="text-xs text-muted-foreground">{t('gentasks.filters.dragDisabled')}</p>
        ) : null}
      </div>
      <BoardCanvas
        board={board.data}
        visibleBoard={filterBoardView(board.data, filter)}
        abilities={abilities}
        taskDragEnabled={taskDragEnabled}
        users={users.data}
        actions={{
          onAddTask: (column) => setTaskDialog({ kind: 'create', columnToken: column.token }),
          onOpenTask: (task) => setTaskDialog({ kind: 'edit', task }),
          onRename: (column) => openColumnEditing({ kind: 'rename', column }),
          onResetName: (column) => resetColumnName.mutate(column),
          onDelete: setColumnDeleting,
        }}
      />
      <TaskDialog
        boardToken={boardToken}
        mode={taskDialog}
        labels={board.data.labels}
        users={users.data}
        abilities={abilities}
        onClose={() => setTaskDialog(null)}
      />
      <NameDialog
        open={columnEditing !== null}
        title={columnEditing?.kind === 'rename' ? t('gentasks.board.renameColumnTitle') : t('gentasks.board.addColumnTitle')}
        label={t('gentasks.board.columnName')}
        initialName={columnEditing?.kind === 'rename' ? resolveColumnTitle(columnEditing.column, t) : ''}
        submitLabel={columnEditing?.kind === 'rename' ? t('common.save') : t('gentasks.board.addColumn')}
        maxLength={60}
        isSaving={saveColumnName.isPending}
        error={saveColumnName.error}
        onSubmit={(name) => saveColumnName.mutate(name)}
        onOpenChange={(open) => (open ? undefined : setColumnEditing(null))}
      />
      <ConfirmDialog
        open={columnDeleting !== null}
        title={t('gentasks.board.deleteColumnTitle')}
        description={t('gentasks.board.deleteColumnDescription', { title: columnDeleting ? resolveColumnTitle(columnDeleting, t) : '' })}
        confirmLabel={t('common.delete')}
        onConfirm={() => columnDeleting && removeColumn.mutate(columnDeleting)}
        onOpenChange={(open) => (open ? undefined : setColumnDeleting(null))}
      />
      <LabelManagerDialog boardToken={boardToken} labels={board.data.labels} open={labelsOpen} onOpenChange={setLabelsOpen} />
      <ArchivedTasksDialog boardToken={boardToken} abilities={abilities} open={archivedOpen} onOpenChange={setArchivedOpen} />
    </section>
  )
}
