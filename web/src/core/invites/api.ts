import { apiRequest } from '../../shared/api/client'
import type { SignInResponse } from '../auth/api'

export type InviteStatus = 'pending' | 'accepted' | 'revoked' | 'expired'

export type Invite = {
  token: string
  email: string
  role: { token: string; name: string }
  invitedBy: { token: string; name: string }
  status: InviteStatus
  createdAt: string
  expiresAt: string
}

export type InviteDelivery = {
  invite: Invite
  link: string
  emailSent: boolean
}

export type Role = { token: string; name: string; permissions: string[] }

export type Invitation = { instanceName: string; email: string; expiresAt: string }

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
