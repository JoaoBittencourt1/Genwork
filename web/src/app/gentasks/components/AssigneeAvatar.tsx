import { UserRoundIcon } from 'lucide-react'
import type { DirectoryUser } from '@/shared/datasource/users'
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
      className={cn(
        'flex size-5 shrink-0 items-center justify-center rounded-full bg-primary/20 text-[9px] font-semibold text-primary',
        className,
      )}
    >
      {initials(assignee.name)}
    </span>
  )
}
