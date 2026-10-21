import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ArchiveIcon, Columns3Icon, PlusIcon, SettingsIcon } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/shared/components/ConfirmDialog'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { PageContent } from '@/shared/components/PageContent'
import { PageHeader } from '@/shared/components/PageHeader'
import { useErrorMessage } from '@/shared/hooks/useErrorMessage'
import { Button } from '@/shared/ui/button'
import { Spinner } from '@/shared/ui/spinner'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/shared/ui/tooltip'
import { ArchivedTasksDialog } from './components/ArchivedTasksDialog'
import { BoardBreadcrumb } from './components/BoardBreadcrumb'
import { BoardCanvas } from './components/BoardCanvas'
import { ActiveFilterBar, BoardFilterMenu } from './components/BoardFilters'
import { GentasksNoPermission } from './components/GentasksNoPermission'
import { NameDialog } from './components/NameDialog'
import { gentasksQueryKeys } from './components/queryKeys'
import { TaskDialog, type TaskDialogMode } from './components/TaskDialog'
import { useDirectoryUsers } from './components/useDirectoryUsers'
import { useGentasksAbilities } from './components/useGentasksAbilities'
import { addColumn, deleteColumn, readBoard, renameColumn } from './datasource/boardsDatasource'
import { resolveColumnTitle } from './domain/columnName'
import { isTaskDragEnabled } from './domain/dragAndDrop'
import { EMPTY_FILTER, filterBoardView, type TaskFilter } from './domain/filters'
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

  if (!abilities.canRead) return <GentasksNoPermission />
  if (board.isPending || board.isError) {
    return (
      <>
        <PageHeader>
          <BoardBreadcrumb />
        </PageHeader>
        <PageContent>{board.isPending ? <Spinner /> : <ErrorAlert error={board.error} />}</PageContent>
      </>
    )
  }

  function openColumnEditing(next: ColumnNameEditing) {
    saveColumnName.reset()
    setColumnEditing(next)
  }

  const firstColumn = board.data.columns[0]

  return (
    <>
      <PageHeader
        actions={
          <>
            <BoardFilterMenu filter={filter} labels={board.data.labels} users={users.data} onChange={setFilter} />
            <IconAction label={t('gentasks.board.archived')} onClick={() => setArchivedOpen(true)}>
              <ArchiveIcon />
            </IconAction>
            {abilities.canManageBoards ? (
              <>
                <IconAction label={t('gentasks.board.addColumn')} onClick={() => openColumnEditing({ kind: 'add' })}>
                  <Columns3Icon />
                </IconAction>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button variant="ghost" size="icon-sm" asChild>
                      <Link to="/tasks/$boardToken/settings" params={{ boardToken }} aria-label={t('gentasks.settings.title')}>
                        <SettingsIcon />
                      </Link>
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{t('gentasks.settings.title')}</TooltipContent>
                </Tooltip>
              </>
            ) : null}
            {abilities.canWriteTasks && firstColumn ? (
              <Button size="sm" className="ml-1" onClick={() => setTaskDialog({ kind: 'create', columnToken: firstColumn.token })}>
                <PlusIcon />
                <span className="hidden sm:inline">{t('gentasks.board.newTask')}</span>
              </Button>
            ) : null}
          </>
        }
      >
        <BoardBreadcrumb board={board.data} />
      </PageHeader>
      <ActiveFilterBar filter={filter} labels={board.data.labels} users={users.data} onChange={setFilter} />
      <BoardCanvas
        board={board.data}
        visibleBoard={filterBoardView(board.data, filter)}
        abilities={abilities}
        taskDragEnabled={isTaskDragEnabled(abilities.canWriteTasks, filter)}
        users={users.data}
        actions={{
          onAddTask: (column) => setTaskDialog({ kind: 'create', columnToken: column.token }),
          onOpenTask: (task) => setTaskDialog({ kind: 'edit', task }),
          onRename: (column) => openColumnEditing({ kind: 'rename', column }),
          onResetName: (column) => resetColumnName.mutate(column),
          onDelete: setColumnDeleting,
        }}
      />
      <TaskDialog board={board.data} mode={taskDialog} users={users.data} abilities={abilities} onClose={() => setTaskDialog(null)} />
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
      <ArchivedTasksDialog
        boardToken={boardToken}
        taskPrefix={board.data.taskPrefix}
        abilities={abilities}
        open={archivedOpen}
        onOpenChange={setArchivedOpen}
      />
    </>
  )
}

function IconAction({ label, onClick, children }: { label: string; onClick: () => void; children: ReactNode }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={label} onClick={onClick}>
          {children}
        </Button>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  )
}
