import type { Role } from '@/shared/domain/role'
import { apiRequest } from './httpClient'

export async function listRoles(): Promise<Role[]> {
  return (await apiRequest<{ roles: Role[] }>('GET', '/api/core/v1/roles')).roles
}
