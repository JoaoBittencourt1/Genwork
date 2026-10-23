import type { DraggableAttributes } from '@dnd-kit/core'
import { useSortable } from '@dnd-kit/sortable'
import { GripVerticalIcon } from 'lucide-react'
import { memo } from 'react'
import { useTranslation } from 'react-i18next'
import type { DirectoryUser } from '@/shared/datasource/users'
import { cn } from '@/shared/lib/utils'
import { canAssignFromCard, findAssignee } from '../domain/assignees'
import { taskDragId } from '../domain/dragAndDrop'
import { haveSameContent } from '../domain/ordering'
import { taskIdentifier } from '../domain/taskPrefix'
import type { Task } from '../domain/types'
import { isUrgencyShown, urgencyTranslationKey } from '../domain/urgency'
import { AssigneeAvatar } from './AssigneeAvatar'
import { LabelChip } from './LabelChip'
import { sortableStyle } from './sortableStyle'
import { TaskAssigneeMenu, type AssigneeChange } from './TaskAssigneeMenu'
import { UrgencyIcon } from './UrgencyIcon'

type TaskCardProps = {
  task: Task
  taskPrefix: string
  users: DirectoryUser[] | undefined
  dragEnabled: boolean
  onOpen: (task: Task) => void
  // onAssign is set only for people who can write tasks; it makes the avatar a menu.
  onAssign?: (change: AssigneeChange) => void
}

const gripIcon = <GripVerticalIcon className="size-4" />

// The whole card drags with the pointer. The keyboard sensor only starts from the activator, so Enter on the card opens it
// and keyboard users pick the card up through the grip, which only shows on focus.
// dnd-kit re-renders every sortable card on each pointer move, so this wrapper stays minimal and its children are memoized.
export const TaskCard = memo(function TaskCard({ task, taskPrefix, users, dragEnabled, onOpen, onAssign }: TaskCardProps) {
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({
    id: taskDragId(task.token),
    disabled: !dragEnabled,
  })
  const menuUsers = onAssign && canAssignFromCard(true, users) ? users : undefined

  return (
    <li
      ref={setNodeRef}
      style={sortableStyle({ transform, transition, isDragging })}
      {...listeners}
      className={dragEnabled ? DRAGGABLE_CARD_CLASS : CARD_CLASS}
    >
      {dragEnabled ? <DragGrip title={task.title} attributes={attributes} setActivatorNodeRef={setActivatorNodeRef} /> : null}
      <TaskCardButton task={task} taskPrefix={taskPrefix} users={users} onOpen={onOpen} avatarInMenu={menuUsers !== undefined} />
      {menuUsers && onAssign ? <TaskAssigneeMenu task={task} users={menuUsers} onAssign={onAssign} /> : null}
    </li>
  )
})

const CARD_CLASS = 'group relative rounded-lg border bg-board-card text-card-foreground shadow-sm transition-colors hover:border-foreground/20'
const DRAGGABLE_CARD_CLASS = cn(CARD_CLASS, 'cursor-grab active:cursor-grabbing')

type DragGripProps = {
  title: string
  attributes: DraggableAttributes
  setActivatorNodeRef: (element: HTMLElement | null) => void
}

const DragGrip = memo(function DragGrip({ title, attributes, setActivatorNodeRef }: DragGripProps) {
  const { t } = useTranslation()
  return (
    <button
      type="button"
      className="absolute top-3 right-9 rounded text-muted-foreground opacity-0 focus-visible:opacity-100 focus-visible:ring-2 focus-visible:ring-ring"
      ref={setActivatorNodeRef}
      aria-label={t('gentasks.task.dragHandle', { title })}
      {...attributes}
    >
      {gripIcon}
    </button>
  )
})

type TaskCardButtonProps = Omit<TaskCardProps, 'dragEnabled' | 'onAssign'> & { avatarInMenu: boolean }

const TaskCardButton = memo(
  function TaskCardButton({ task, taskPrefix, users, onOpen, avatarInMenu }: TaskCardButtonProps) {
    return (
      <button type="button" className="flex w-full flex-col gap-1.5 p-3 text-left" onClick={() => onOpen(task)}>
        <TaskCardBody task={task} taskPrefix={taskPrefix} users={users} avatarInMenu={avatarInMenu} />
      </button>
    )
  },
  (previous, next) =>
    haveSameContent(previous.task, next.task) &&
    previous.taskPrefix === next.taskPrefix &&
    previous.users === next.users &&
    previous.avatarInMenu === next.avatarInMenu &&
    previous.onOpen === next.onOpen,
)

// TaskCardPreview follows the pointer while a card is dragged; it lives outside the columns, so their scroll never clips it.
export function TaskCardPreview({ task, taskPrefix, users }: Omit<TaskCardProps, 'dragEnabled' | 'onOpen' | 'onAssign'>) {
  return (
    <div className="flex cursor-grabbing flex-col gap-1.5 rounded-lg border border-foreground/20 bg-board-card p-3 text-card-foreground shadow-lg">
      <TaskCardBody task={task} taskPrefix={taskPrefix} users={users} />
    </div>
  )
}

type TaskCardBodyProps = Omit<TaskCardProps, 'dragEnabled' | 'onOpen' | 'onAssign'> & { avatarInMenu?: boolean }

// With avatarInMenu the avatar is drawn by TaskAssigneeMenu, over the space kept here.
const TaskCardBody = memo(function TaskCardBody({ task, taskPrefix, users, avatarInMenu = false }: TaskCardBodyProps) {
  const { t, i18n } = useTranslation()
  const createdAt = shortDateFormat(i18n.language).format(new Date(task.createdAt))

  return (
    <>
      <span className="flex items-center justify-between gap-2">
        <span className="text-xs text-muted-foreground tabular-nums">{taskIdentifier(taskPrefix, task.number)}</span>
        {avatarInMenu ? (
          <span className="size-5" aria-hidden />
        ) : (
          <AssigneeAvatar assignee={findAssignee(users, task.assigneeToken)} unassignedLabel={t('gentasks.task.unassigned')} />
        )}
      </span>
      <span className="line-clamp-3 text-sm font-medium break-words">{task.title}</span>
      {isUrgencyShown(task.urgency) || task.labels.length > 0 ? (
        <span className="mt-1 flex flex-wrap items-center gap-1">
          {isUrgencyShown(task.urgency) ? (
            <span className="inline-flex h-6 items-center rounded-md border px-1.5" title={t(urgencyTranslationKey(task.urgency))}>
              <UrgencyIcon urgency={task.urgency} />
              <span className="sr-only">{t(urgencyTranslationKey(task.urgency))}</span>
            </span>
          ) : null}
          {task.labels.map((label) => (
            <LabelChip key={label.token} label={label} />
          ))}
        </span>
      ) : null}
      <span className="mt-1 text-xs text-muted-foreground">{t('gentasks.task.createdAt', { date: createdAt })}</span>
    </>
  )
})

const shortDateFormats = new Map<string, Intl.DateTimeFormat>()

function shortDateFormat(language: string): Intl.DateTimeFormat {
  let format = shortDateFormats.get(language)
  if (!format) {
    format = new Intl.DateTimeFormat(language, { month: 'short', day: 'numeric' })
    shortDateFormats.set(language, format)
  }
  return format
}
