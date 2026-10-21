import type { DirectoryUser } from '@/shared/datasource/users'

export function findAssignee(users: DirectoryUser[] | undefined, assigneeToken: string | null): DirectoryUser | undefined {
  if (!assigneeToken) return undefined
  return users?.find((user) => user.token === assigneeToken)
}

export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  const letters = parts.length > 1 ? [parts[0]?.[0], parts[parts.length - 1]?.[0]] : [parts[0]?.[0], parts[0]?.[1]]
  return letters.filter(Boolean).join('').toUpperCase()
}
