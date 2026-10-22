export type Role = { token: string; name: string; permissions: string[] }

// Mirrors core's rule: a role may only be handed out by someone whose own role grants all of its permissions.
export function isRoleCoveredBy(role: Role, permissions: readonly string[]): boolean {
  return role.permissions.every((permission) => permissions.includes(permission))
}

export function assignableRoles(roles: Role[], permissions: readonly string[]): Role[] {
  return roles.filter((role) => isRoleCoveredBy(role, permissions))
}
