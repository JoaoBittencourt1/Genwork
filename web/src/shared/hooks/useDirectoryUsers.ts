import { useQuery } from '@tanstack/react-query'
import { useAuth } from '@/app/auth/components/AuthContext'
import { PERMISSIONS } from '@/app/auth/domain/session'
import { listUsers } from '@/shared/datasource/users'
import { directoryUsersQueryKey } from './queryKeys'

export function useDirectoryUsers() {
  const { can } = useAuth()
  return useQuery({ queryKey: directoryUsersQueryKey, queryFn: listUsers, enabled: can(PERMISSIONS.userRead) })
}
