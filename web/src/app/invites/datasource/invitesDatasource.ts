import type { SignInResponse } from '@/app/auth/datasource/authDatasource'
import { apiRequest } from '@/shared/datasource/httpClient'
import type { Invitation, Invite, InviteDelivery, Role } from '../domain/invite'

export async function listInvites(): Promise<Invite[]> {
  return (await apiRequest<{ invites: Invite[] }>('GET', '/api/core/v1/invites')).invites
}

export async function listRoles(): Promise<Role[]> {
  return (await apiRequest<{ roles: Role[] }>('GET', '/api/core/v1/roles')).roles
}

export function createInvite(email: string, roleToken: string): Promise<InviteDelivery> {
  return apiRequest<InviteDelivery>('POST', '/api/core/v1/invites', { body: { email, role: roleToken } })
}

export function resendInvite(inviteToken: string): Promise<InviteDelivery> {
  return apiRequest<InviteDelivery>('POST', `/api/core/v1/invites/${encodeURIComponent(inviteToken)}/resend`)
}

export function revokeInvite(inviteToken: string): Promise<void> {
  return apiRequest<void>('DELETE', `/api/core/v1/invites/${encodeURIComponent(inviteToken)}`)
}

export function inspectInvitation(secret: string): Promise<Invitation> {
  return apiRequest<Invitation>('GET', `/api/core/v1/invitations/${encodeURIComponent(secret)}`, { authenticated: false })
}

export function acceptInvitation(secret: string, name: string, password: string): Promise<SignInResponse> {
  return apiRequest<SignInResponse>('POST', `/api/core/v1/invitations/${encodeURIComponent(secret)}/accept`, {
    body: { name, password, client: 'web' },
    authenticated: false,
  })
}
