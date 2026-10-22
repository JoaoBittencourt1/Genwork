import { CheckIcon } from 'lucide-react'
import { memo } from 'react'
import { useTranslation } from 'react-i18next'
import type { DirectoryUser } from '@/shared/datasource/users'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/shared/ui/dropdown-menu'
import { findAssignee } from '../domain/assignees'
import type { Task } from '../domain/types'
import { AssigneeAvatar } from './AssigneeAvatar'

export type AssigneeChange = { taskToken: string; assigneeToken: string | null }

type TaskAssigneeMenuProps = {
  task: Task
  users: DirectoryUser[]
  onAssign: (change: AssigneeChange) => void
}

// The card's avatar opens this menu, so the assignee changes without opening the task. It sits beside the card's own
// button, not inside it, and keeps pointer events away from the card: they would open the task or start a drag.
export const TaskAssigneeMenu = memo(function TaskAssigneeMenu({ task, users, onAssign }: TaskAssigneeMenuProps) {
  const { t } = useTranslation()
  const unassignedLabel = t('gentasks.task.unassigned')
  const keepFromCard = (event: { stopPropagation: () => void }) => event.stopPropagation()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={t('gentasks.task.changeAssignee', { title: task.title })}
        className="absolute top-3 right-3 rounded-full focus-visible:ring-2 focus-visible:ring-ring"
        onPointerDown={keepFromCard}
        onClick={keepFromCard}
      >
        <AssigneeAvatar assignee={findAssignee(users, task.assigneeToken)} unassignedLabel={unassignedLabel} />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" onPointerDown={keepFromCard} onClick={keepFromCard} onKeyDown={keepFromCard}>
        <AssigneeOption selected={task.assigneeToken === null} onSelect={() => onAssign({ taskToken: task.token, assigneeToken: null })}>
          <AssigneeAvatar assignee={undefined} unassignedLabel={unassignedLabel} />
          {unassignedLabel}
        </AssigneeOption>
        {users.map((user) => (
          <AssigneeOption key={user.token} selected={task.assigneeToken === user.token} onSelect={() => onAssign({ taskToken: task.token, assigneeToken: user.token })}>
            <AssigneeAvatar assignee={user} unassignedLabel={unassignedLabel} />
            {user.name}
          </AssigneeOption>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
})

function AssigneeOption({ selected, onSelect, children }: { selected: boolean; onSelect: () => void; children: React.ReactNode }) {
  return (
    <DropdownMenuItem onSelect={onSelect}>
      {children}
      {selected ? <CheckIcon className="ml-auto size-4" aria-hidden /> : null}
    </DropdownMenuItem>
  )
}
