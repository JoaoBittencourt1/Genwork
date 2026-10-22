import type { AvatarColor } from '@/shared/domain/avatarColor'
import { isRoleCoveredBy, type Role } from '@/shared/domain/role'

export type MemberStatus = 'active' | 'deactivated'

export type Member = {
  token: string
  name: string
  email: string
  avatarColor: AvatarColor
  role: { token: string; name: string }
  status: MemberStatus
  joinedAt: string
  lastSignInAt: string | null
  restrictedModules: string[]
}

export type MemberChanges = {
  role?: string
  status?: MemberStatus
  restrictedModules?: string[]
}

// rolePermissions are the viewer's role permissions, not their effective ones, because core compares roles.
export type MemberViewer = { token: string; rolePermissions: readonly string[]; canManage: boolean }

export type MemberActions = {
  isSelf: boolean
  canChangeRole: boolean
  canChangeModules: boolean
  canDeactivate: boolean
  canReactivate: boolean
  canRemove: boolean
}

export function memberActions(member: Member, viewer: MemberViewer, roles: Role[]): MemberActions {
  const isSelf = member.token === viewer.token
  const memberRole = roles.find((role) => role.token === member.role.token)
  const manageable = viewer.canManage && !isSelf && memberRole !== undefined && isRoleCoveredBy(memberRole, viewer.rolePermissions)
  return {
    isSelf,
    canChangeRole: manageable,
    canChangeModules: manageable,
    canDeactivate: manageable && member.status === 'active',
    canReactivate: manageable && member.status === 'deactivated',
    canRemove: manageable,
  }
}

export type ModuleAccessSummary = { kind: 'all' } | { kind: 'none' } | { kind: 'some'; moduleIds: string[] }

export function moduleAccessSummary(enabledModuleIds: readonly string[], restrictedModules: readonly string[]): ModuleAccessSummary {
  const accessible = enabledModuleIds.filter((moduleId) => !restrictedModules.includes(moduleId))
  if (accessible.length === enabledModuleIds.length) return { kind: 'all' }
  if (accessible.length === 0) return { kind: 'none' }
  return { kind: 'some', moduleIds: accessible }
}

export function moduleAccessByModule(listedModuleIds: readonly string[], restrictedModules: readonly string[]): Record<string, boolean> {
  return Object.fromEntries(listedModuleIds.map((moduleId) => [moduleId, !restrictedModules.includes(moduleId)]))
}

// toRestrictedModules keeps restrictions on modules the dialog did not list, so saving never lifts them by accident.
export function toRestrictedModules(
  previousRestricted: readonly string[],
  listedModuleIds: readonly string[],
  accessByModule: Readonly<Record<string, boolean>>,
): string[] {
  const unlisted = previousRestricted.filter((moduleId) => !listedModuleIds.includes(moduleId))
  const switchedOff = listedModuleIds.filter((moduleId) => accessByModule[moduleId] === false)
  return [...unlisted, ...switchedOff].sort()
}
