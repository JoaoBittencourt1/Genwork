import type { AvatarColor } from '@/shared/domain/avatarColor'
import { apiRequest } from './httpClient'

export type MemberKind = 'human' | 'bot'

export type DirectoryUser = { token: string; kind: MemberKind; name: string; email: string; avatarColor: AvatarColor }

export async function listUsers(): Promise<DirectoryUser[]> {
  return (await apiRequest<{ users: DirectoryUser[] }>('GET', '/api/core/v1/users')).users
}
