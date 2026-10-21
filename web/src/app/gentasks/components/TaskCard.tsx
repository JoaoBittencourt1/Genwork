import { useSortable } from '@dnd-kit/sortable'
import { GripVerticalIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { DirectoryUser } from '@/shared/datasource/users'
import { findAssignee, initials } from '../domain/assignees'
import { taskDragId } from '../domain/dragAndDrop'
import type { Task } from '../domain/types'
import { isUrgencyShown } from '../domain/urgency'
import { LabelChip } from './LabelChip'
import { sortableStyle } from './sortableStyle'
import { UrgencyIndicator } from './UrgencyIndicator'

type TaskCardProps = {
  task: Task
  users: DirectoryUser[] | undefined
  dragEnabled: boolean
  onOpen: (task: Task) => void
}

export function TaskCard({ task, users, dragEnabled, onOpen }: TaskCardProps) {
  const { t } = useTranslation()
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: taskDragId(task.token),
    disabled: !dragEnabled,
  })
  const assignee = findAssignee(users, task.assigneeToken)

  return (
    <li ref={setNodeRef} style={sortableStyle({ transform, transition, isDragging })} className="flex gap-1 rounded-lg border bg-card p-2 shadow-xs">
      {dragEnabled ? (
        <button
          type="button"
          className="mt-0.5 h-fit cursor-grab rounded text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={t('gentasks.task.dragHandle', { title: task.title })}
          {...attributes}
          {...listeners}
        >
          <GripVerticalIcon className="size-4" />
        </button>
      ) : null}
      <button type="button" className="flex min-w-0 flex-1 flex-col gap-2 text-left" onClick={() => onOpen(task)}>
        <span className="text-sm font-medium break-words">{task.title}</span>
        {task.labels.length > 0 ? (
          <span className="flex flex-wrap gap-1">
            {task.labels.map((label) => (
              <LabelChip key={label.token} label={label} />
            ))}
          </span>
        ) : null}
        {isUrgencyShown(task.urgency) || assignee ? (
          <span className="flex items-center justify-between gap-2">
            {isUrgencyShown(task.urgency) ? <UrgencyIndicator urgency={task.urgency} /> : <span />}
            {assignee ? (
              <span
                className="flex size-6 items-center justify-center rounded-full bg-secondary text-[10px] font-semibold text-secondary-foreground"
                title={assignee.name}
              >
                {initials(assignee.name)}
              </span>
            ) : null}
          </span>
        ) : null}
      </button>
    </li>
  )
}
