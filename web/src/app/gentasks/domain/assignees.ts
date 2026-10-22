import type { DirectoryUser } from '@/shared/datasource/users'

export function findAssignee(users: DirectoryUser[] | undefined, assigneeToken: string | null): DirectoryUser | undefined {
  if (!assigneeToken) return undefined
  return users?.find((user) => user.token === assigneeToken)
}
