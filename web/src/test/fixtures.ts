import type { Me } from '../core/auth/api'
import type { Invite, InviteDelivery, Role } from '../core/invites/api'

export const discoveryFixture = {
  product: 'genwork',
  name: 'Genwork da Empresa',
  version: '0.1.0',
  api: 'v1',
  auth: { methods: ['password'] },
}

const allInvitePermissions = ['core.invite.create', 'core.invite.read', 'core.invite.revoke']

export const adminFixture: Me = {
  token: 'user-ana',
  email: 'ana@example.com',
  name: 'Ana',
  role: { token: 'role-admin', name: 'admin', permissions: allInvitePermissions },
  permissions: allInvitePermissions,
}

export const memberFixture: Me = {
  token: 'user-bruno',
  email: 'bruno@example.com',
  name: 'Bruno',
  role: { token: 'role-member', name: 'member', permissions: [] },
  permissions: [],
}

export const rolesFixture: Role[] = [
  { token: 'role-admin', name: 'admin', permissions: allInvitePermissions },
  { token: 'role-member', name: 'member', permissions: [] },
]

export function inviteFixture(overrides: Partial<Invite> = {}): Invite {
  return {
    token: 'invite-carla',
    email: 'carla@example.com',
    role: { token: 'role-member', name: 'member' },
    invitedBy: { token: 'user-ana', name: 'Ana' },
    status: 'pending',
    createdAt: '2026-10-20T12:00:00Z',
    expiresAt: '2026-10-27T12:00:00Z',
    ...overrides,
  }
}

export function deliveryFixture(overrides: Partial<InviteDelivery> = {}): InviteDelivery {
  return {
    invite: inviteFixture(),
    link: 'http://localhost:8485/invite/secret-carla',
    emailSent: true,
    ...overrides,
  }
}

export const invitationFixture = {
  instanceName: 'Genwork da Empresa',
  email: 'carla@example.com',
  expiresAt: '2026-10-27T12:00:00Z',
}
