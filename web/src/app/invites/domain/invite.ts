import { PERMISSIONS } from '@/app/auth/domain/session'
import type { Role } from '@/shared/domain/role'

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

export type Invitation = { instanceName: string; email: string; expiresAt: string }

export type InviteActions = { canResend: boolean; canRevoke: boolean }

export type PermissionCheck = (permission: string) => boolean

const DEFAULT_ROLE_NAME = 'member'
const RESENDABLE_STATUSES: InviteStatus[] = ['pending', 'expired']

export function inviteActions(invite: Invite, can: PermissionCheck): InviteActions {
  return {
    canResend: can(PERMISSIONS.inviteCreate) && RESENDABLE_STATUSES.includes(invite.status),
    canRevoke: can(PERMISSIONS.inviteRevoke) && invite.status === 'pending',
  }
}

export function pickDefaultRole(roles: Role[]): Role | undefined {
  return roles.find((role) => role.name === DEFAULT_ROLE_NAME) ?? roles[0]
}

export function passwordsMatch(password: string, confirmation: string): boolean {
  return password === confirmation
}
