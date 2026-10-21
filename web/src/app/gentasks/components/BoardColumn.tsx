import { SortableContext, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { GripVerticalIcon, MoreHorizontalIcon, PlusIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { DirectoryUser } from '@/shared/datasource/users'
import { Button } from '@/shared/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/shared/ui/dropdown-menu'
import { canClearColumnName, resolveColumnTitle } from '../domain/columnName'
import { columnDragId, taskDragId } from '../domain/dragAndDrop'
import type { GentasksAbilities } from '../domain/permissions'
import type { ColumnWithTasks, Task } from '../domain/types'
import { sortableStyle } from './sortableStyle'
import { TaskCard } from './TaskCard'

export type ColumnActions = {
  onAddTask: (column: ColumnWithTasks) => void
  onOpenTask: (task: Task) => void
  onRename: (column: ColumnWithTasks) => void
  onResetName: (column: ColumnWithTasks) => void
  onDelete: (column: ColumnWithTasks) => void
}

type BoardColumnProps = {
  column: ColumnWithTasks
  abilities: GentasksAbilities
  taskDragEnabled: boolean
  users: DirectoryUser[] | undefined
  actions: ColumnActions
}

export function BoardColumn({ column, abilities, taskDragEnabled, users, actions }: BoardColumnProps) {
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
      className="flex max-h-full w-72 shrink-0 flex-col gap-2 rounded-xl bg-muted/60 p-2"
    >
      <header className="flex items-center gap-1 px-1">
        {abilities.canManageBoards ? (
          <button
            type="button"
            className="cursor-grab rounded text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
            aria-label={t('gentasks.board.dragColumn', { title })}
            {...attributes}
            {...listeners}
          >
            <GripVerticalIcon className="size-4" />
          </button>
        ) : null}
        <h2 className="truncate text-sm font-semibold">{title}</h2>
        <span className="text-xs text-muted-foreground">{column.tasks.length}</span>
        {abilities.canManageBoards ? (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon-sm" className="ml-auto" aria-label={t('gentasks.board.columnActions', { title })}>
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
      </header>
      <SortableContext items={column.tasks.map((task) => taskDragId(task.token))} strategy={verticalListSortingStrategy}>
        <ul className="flex min-h-12 flex-col gap-2 overflow-y-auto">
          {column.tasks.map((task) => (
            <TaskCard key={task.token} task={task} users={users} dragEnabled={taskDragEnabled} onOpen={actions.onOpenTask} />
          ))}
        </ul>
      </SortableContext>
      {abilities.canWriteTasks ? (
        <Button variant="ghost" size="sm" className="justify-start text-muted-foreground" onClick={() => actions.onAddTask(column)}>
          <PlusIcon />
          {t('gentasks.board.addTask')}
        </Button>
      ) : null}
    </section>
  )
}
