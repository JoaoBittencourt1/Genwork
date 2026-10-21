import { useQuery } from '@tanstack/react-query'
import { listUsers } from '@/shared/datasource/users'

export const directoryUsersQueryKey = ['core', 'directory-users'] as const

// useDirectoryUsers only fetches when the caller may read the directory (core.user.read).
export function useDirectoryUsers(canReadDirectory: boolean) {
  return useQuery({ queryKey: directoryUsersQueryKey, queryFn: listUsers, enabled: canReadDirectory })
}
