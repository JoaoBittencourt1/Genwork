import type { Member } from '@/app/members/domain/member'
import type { Role } from '@/shared/domain/role'

export const adminRolePermissions = ['core.invite.create', 'core.member.manage', 'core.member.read', 'core.user.read', 'gentasks.board.read']
export const memberRolePermissions = ['core.user.read', 'gentasks.board.read']

export const memberRolesFixture: Role[] = [
  { token: 'role-admin', name: 'admin', permissions: adminRolePermissions },
  { token: 'role-member', name: 'member', permissions: memberRolePermissions },
]

export function memberFixture(overrides: Partial<Member> = {}): Member {
  return {
    token: 'user-bruno',
    name: 'Bruno',
    email: 'bruno@example.com',
    avatarColor: 'pink',
    role: { token: 'role-member', name: 'member' },
    status: 'active',
    joinedAt: '2026-10-01T12:00:00Z',
    lastSignInAt: '2026-10-21T09:00:00Z',
    restrictedModules: [],
    ...overrides,
  }
}
