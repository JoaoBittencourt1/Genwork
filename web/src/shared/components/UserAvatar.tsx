import { avatarColorVariable, type AvatarColor } from '@/shared/domain/avatarColor'
import { initials } from '@/shared/domain/initials'
import { cn } from '@/shared/lib/utils'

type UserAvatarProps = {
  name: string
  avatarColor: AvatarColor
  className?: string
}

export function UserAvatar({ name, avatarColor, className }: UserAvatarProps) {
  return (
    <span
      title={name}
      style={{ backgroundColor: avatarColorVariable(avatarColor) }}
      className={cn(
        'flex size-8 shrink-0 items-center justify-center rounded-full text-xs font-semibold text-avatar-foreground ring-1 ring-border',
        className,
      )}
    >
      {initials(name)}
    </span>
  )
}
