export type Me = {
  token: string
  email: string
  name: string
  role: { token: string; name: string; permissions: string[] }
  permissions: string[]
}

export type AuthState = { status: 'loading' } | { status: 'signedOut' } | { status: 'signedIn'; me: Me }

export const PERMISSIONS = {
  inviteCreate: 'core.invite.create',
  inviteRead: 'core.invite.read',
  inviteRevoke: 'core.invite.revoke',
  userRead: 'core.user.read',
  memberRead: 'core.member.read',
  memberManage: 'core.member.manage',
  systemUpdate: 'core.system.update',
} as const

export type SessionGateway = {
  refresh: () => Promise<boolean>
  fetchMe: () => Promise<Me>
}

export function hasPermission(state: AuthState, permission: string): boolean {
  return state.status === 'signedIn' && state.me.permissions.includes(permission)
}

export async function restoreSession(gateway: SessionGateway): Promise<AuthState> {
  if (!(await gateway.refresh())) {
    return { status: 'signedOut' }
  }
  try {
    return { status: 'signedIn', me: await gateway.fetchMe() }
  } catch {
    return { status: 'signedOut' }
  }
}
