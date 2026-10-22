import { SortableContext, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { MoreHorizontalIcon, PlusIcon } from 'lucide-react'
import { memo } from 'react'
import { useTranslation } from 'react-i18next'
import type { DirectoryUser } from '@/shared/datasource/users'
import { cn } from '@/shared/lib/utils'
import { Button } from '@/shared/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/shared/ui/dropdown-menu'
import { canClearColumnName, resolveColumnTitle } from '../domain/columnName'
import { columnDragId, taskDragId, taskListId } from '../domain/dragAndDrop'
import type { GentasksAbilities } from '../domain/permissions'
import type { ColumnWithTasks, Task } from '../domain/types'
import { ColumnStatusIcon } from './ColumnStatusIcon'
import { sortableStyle } from './sortableStyle'
import { TaskCard } from './TaskCard'
import type { AssigneeChange } from './TaskAssigneeMenu'

export type ColumnActions = {
  onAddTask: (column: ColumnWithTasks) => void
  onOpenTask: (task: Task) => void
  onRename: (column: ColumnWithTasks) => void
  onResetName: (column: ColumnWithTasks) => void
  onDelete: (column: ColumnWithTasks) => void
}

type BoardColumnProps = {
  column: ColumnWithTasks
  taskPrefix: string
  abilities: GentasksAbilities
  taskDragEnabled: boolean
  users: DirectoryUser[] | undefined
  actions: ColumnActions
  onAssign?: (change: AssigneeChange) => void
}

// The column header is its drag handle, so columns reorder by grabbing their title like in Linear.
// dnd-kit re-renders every sortable column on each pointer move, so everything below the wrapper is memoized.
export const BoardColumn = memo(function BoardColumn({ column, taskPrefix, abilities, taskDragEnabled, users, actions, onAssign }: BoardColumnProps) {
  const { t } = useTranslation()
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: columnDragId(column.token),
    disabled: !abilities.canManageBoards,
  })
  const title = resolveColumnTitle(column, t)

  return (
    <section
      ref={setNodeRef}
      style={sortableStyle({ transform, transition, isDragging })}
      aria-label={title}
      className="flex max-h-full w-80 shrink-0 flex-col rounded-xl bg-muted/40"
    >
      <header className="flex h-11 shrink-0 items-center gap-1 pr-1.5 pl-3">
        <div
          className={cn(
            'flex min-w-0 flex-1 items-center gap-2 rounded focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
            abilities.canManageBoards && 'cursor-grab active:cursor-grabbing',
          )}
          aria-label={abilities.canManageBoards ? t('gentasks.board.dragColumn', { title }) : undefined}
          {...(abilities.canManageBoards ? { ...attributes, ...listeners } : {})}
        >
          <ColumnTitle column={column} title={title} />
        </div>
        <ColumnActionButtons column={column} title={title} abilities={abilities} actions={actions} />
      </header>
      <ColumnTaskList column={column} taskPrefix={taskPrefix} taskDragEnabled={taskDragEnabled} users={users} onOpenTask={actions.onOpenTask} onAssign={onAssign} />
    </section>
  )
})

const ColumnTitle = memo(function ColumnTitle({ column, title }: { column: ColumnWithTasks; title: string }) {
  return (
    <>
      <ColumnStatusIcon column={column} />
      <h2 className="truncate text-sm font-medium">{title}</h2>
      <span className="text-sm text-muted-foreground tabular-nums">{column.tasks.length}</span>
    </>
  )
})

type ColumnActionButtonsProps = {
  column: ColumnWithTasks
  title: string
  abilities: GentasksAbilities
  actions: ColumnActions
}

const ColumnActionButtons = memo(function ColumnActionButtons({ column, title, abilities, actions }: ColumnActionButtonsProps) {
  const { t } = useTranslation()
  return (
    <>
      {abilities.canManageBoards ? (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon-sm" className="text-muted-foreground" aria-label={t('gentasks.board.columnActions', { title })}>
              <MoreHorizontalIcon />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onSelect={() => actions.onRename(column)}>{t('gentasks.board.renameColumn')}</DropdownMenuItem>
            {canClearColumnName(column) ? (
              <DropdownMenuItem onSelect={() => actions.onResetName(column)}>{t('gentasks.board.resetColumnName')}</DropdownMenuItem>
            ) : null}
            <DropdownMenuItem variant="destructive" onSelect={() => actions.onDelete(column)}>
              {t('gentasks.board.deleteColumn')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ) : null}
      {abilities.canWriteTasks ? (
        <Button
          variant="ghost"
          size="icon-sm"
          className="text-muted-foreground"
          aria-label={t('gentasks.board.addTaskTo', { title })}
          onClick={() => actions.onAddTask(column)}
        >
          <PlusIcon />
        </Button>
      ) : null}
    </>
  )
})

type ColumnTaskListProps = {
  column: ColumnWithTasks
  taskPrefix: string
  taskDragEnabled: boolean
  users: DirectoryUser[] | undefined
  onOpenTask: (task: Task) => void
  onAssign?: (change: AssigneeChange) => void
}

const ColumnTaskList = memo(function ColumnTaskList({ column, taskPrefix, taskDragEnabled, users, onOpenTask, onAssign }: ColumnTaskListProps) {
  return (
    <SortableContext id={taskListId(column.token)} items={column.tasks.map((task) => taskDragId(task.token))} strategy={verticalListSortingStrategy}>
      <ul className="flex min-h-16 flex-1 flex-col gap-2 overflow-y-auto px-2 pb-2">
        {column.tasks.map((task) => (
          <TaskCard key={task.token} task={task} taskPrefix={taskPrefix} users={users} dragEnabled={taskDragEnabled} onOpen={onOpenTask} onAssign={onAssign} />
        ))}
      </ul>
    </SortableContext>
  )
})
