import { apiRequest } from './httpClient'

export type DirectoryUser = { token: string; name: string; email: string }

export async function listUsers(): Promise<DirectoryUser[]> {
  return (await apiRequest<{ users: DirectoryUser[] }>('GET', '/api/core/v1/users')).users
}
