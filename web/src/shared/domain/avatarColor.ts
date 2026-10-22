export const AVATAR_COLORS = ['blue', 'light-green', 'pink', 'purple', 'red', 'yellow', 'white', 'orange'] as const
export type AvatarColor = (typeof AVATAR_COLORS)[number]

// Colors come only from theme variables defined in index.css (rule 81).
export function avatarColorVariable(color: AvatarColor): string {
  return `var(--avatar-${color})`
}
