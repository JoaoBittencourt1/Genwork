import { useQuery } from '@tanstack/react-query'
import { listRoles } from '@/shared/datasource/roles'

export const rolesQueryKey = ['core', 'roles'] as const

// useRoles only fetches when the caller may list roles (core.invite.create or core.member.manage).
export function useRoles(canListRoles = true) {
  return useQuery({ queryKey: rolesQueryKey, queryFn: listRoles, enabled: canListRoles })
}
