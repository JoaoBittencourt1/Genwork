import { UserRoundIcon } from 'lucide-react'
import { UserAvatar } from '@/shared/components/UserAvatar'
import type { DirectoryUser } from '@/shared/datasource/users'
import { cn } from '@/shared/lib/utils'

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
  return <UserAvatar name={assignee.name} avatarColor={assignee.avatarColor} className={cn('size-5 text-[9px]', className)} />
}
