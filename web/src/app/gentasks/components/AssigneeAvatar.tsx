import { UserRoundIcon } from 'lucide-react'
import type { DirectoryUser } from '@/shared/datasource/users'
import { avatarColorVariable } from '@/shared/domain/avatarColor'
import { cn } from '@/shared/lib/utils'
import { initials } from '../domain/assignees'

type AssigneeAvatarProps = {
  assignee: DirectoryUser | undefined
  unassignedLabel: string
  className?: string
}

export function AssigneeAvatar({ assignee, unassignedLabel, className }: AssigneeAvatarProps) {
  if (!assignee) {
    return (
      <span
        title={unassignedLabel}
        className={cn('flex size-5 shrink-0 items-center justify-center rounded-full border border-dashed text-muted-foreground', className)}
      >
        <UserRoundIcon className="size-3" aria-hidden />
        <span className="sr-only">{unassignedLabel}</span>
      </span>
    )
  }
  return (
    <span
      title={assignee.name}
      style={{ backgroundColor: avatarColorVariable(assignee.avatarColor) }}
      className={cn(
        'flex size-5 shrink-0 items-center justify-center rounded-full text-[9px] font-semibold text-avatar-foreground ring-1 ring-border',
        className,
      )}
    >
      {initials(assignee.name)}
    </span>
  )
}
