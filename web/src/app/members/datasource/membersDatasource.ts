import { apiRequest } from '@/shared/datasource/httpClient'
import type { Member, MemberChanges } from '../domain/member'

export async function listMembers(): Promise<Member[]> {
  return (await apiRequest<{ members: Member[] }>('GET', '/api/core/v1/members')).members
}

export function updateMember(memberToken: string, changes: MemberChanges): Promise<Member> {
  return apiRequest<Member>('PATCH', `/api/core/v1/members/${encodeURIComponent(memberToken)}`, { body: changes })
}

export function removeMember(memberToken: string): Promise<void> {
  return apiRequest<void>('DELETE', `/api/core/v1/members/${encodeURIComponent(memberToken)}`)
}
