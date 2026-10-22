import { describe, expect, it } from 'vitest'
import { adminRolePermissions, memberRolePermissions, memberRolesFixture } from '@/test/memberFixtures'
import { assignableRoles, isRoleCoveredBy } from './role'

describe('isRoleCoveredBy', () => {
  it('requires every permission of the role', () => {
    const [admin, member] = memberRolesFixture
    expect(isRoleCoveredBy(member!, adminRolePermissions)).toBe(true)
    expect(isRoleCoveredBy(admin!, memberRolePermissions)).toBe(false)
  })
})

describe('assignableRoles', () => {
  it('keeps only the roles the permissions cover', () => {
    expect(assignableRoles(memberRolesFixture, adminRolePermissions).map((role) => role.name)).toEqual(['admin', 'member'])
    expect(assignableRoles(memberRolesFixture, memberRolePermissions).map((role) => role.name)).toEqual(['member'])
  })
})
