import { describe, expect, it } from 'vitest'
import { adminRolePermissions, memberFixture, memberRolePermissions, memberRolesFixture } from '@/test/memberFixtures'
import { memberActions, moduleAccessByModule, moduleAccessSummary, toRestrictedModules, type MemberViewer } from './member'

const adminViewer: MemberViewer = { token: 'user-ana', rolePermissions: adminRolePermissions, canManage: true }
const adminTarget = { role: { token: 'role-admin', name: 'admin' } }

describe('memberActions', () => {
  it('offers everything but deactivate for another, deactivated admin', () => {
    const actions = memberActions(memberFixture({ ...adminTarget, status: 'deactivated' }), adminViewer, memberRolesFixture)

    expect(actions).toEqual({
      isSelf: false,
      canChangeRole: true,
      canChangeModules: true,
      canDeactivate: false,
      canReactivate: true,
      canRemove: true,
    })
  })

  it('offers deactivate but not reactivate for an active member', () => {
    const actions = memberActions(memberFixture(), adminViewer, memberRolesFixture)

    expect(actions.canDeactivate).toBe(true)
    expect(actions.canReactivate).toBe(false)
  })

  it('offers nothing on the viewer own row', () => {
    const actions = memberActions(memberFixture({ token: 'user-ana', ...adminTarget }), adminViewer, memberRolesFixture)

    expect(actions).toEqual({
      isSelf: true,
      canChangeRole: false,
      canChangeModules: false,
      canDeactivate: false,
      canReactivate: false,
      canRemove: false,
    })
  })

  it('offers nothing without core.member.manage', () => {
    const actions = memberActions(memberFixture(), { ...adminViewer, canManage: false }, memberRolesFixture)

    expect(Object.values(actions).some(Boolean)).toBe(false)
  })

  it('offers nothing on a member whose role the viewer does not cover', () => {
    const viewer: MemberViewer = { token: 'user-carla', rolePermissions: memberRolePermissions, canManage: true }

    expect(memberActions(memberFixture(adminTarget), viewer, memberRolesFixture).canRemove).toBe(false)
    expect(memberActions(memberFixture(), viewer, memberRolesFixture).canRemove).toBe(true)
  })

  it('offers nothing while the roles are unknown', () => {
    expect(memberActions(memberFixture(), adminViewer, []).canChangeRole).toBe(false)
  })
})

describe('moduleAccessSummary', () => {
  it('lists the enabled modules the member is not restricted from', () => {
    expect(moduleAccessSummary(['gentasks', 'insights'], ['insights', 'disabled-module'])).toEqual({ kind: 'some', moduleIds: ['gentasks'] })
  })

  it('says all when no enabled module is restricted, and none when every one is', () => {
    expect(moduleAccessSummary(['gentasks'], ['disabled-module'])).toEqual({ kind: 'all' })
    expect(moduleAccessSummary([], [])).toEqual({ kind: 'all' })
    expect(moduleAccessSummary(['gentasks'], ['gentasks'])).toEqual({ kind: 'none' })
  })
})

describe('moduleAccessByModule', () => {
  it('turns on every listed module the member is not restricted from', () => {
    expect(moduleAccessByModule(['gentasks', 'insights'], ['insights', 'disabled-module'])).toEqual({ gentasks: true, insights: false })
  })
})

describe('toRestrictedModules', () => {
  it('restricts the listed modules whose switch is off', () => {
    expect(toRestrictedModules([], ['gentasks', 'insights'], { gentasks: false, insights: true })).toEqual(['gentasks'])
  })

  it('keeps restrictions on modules the dialog does not list, such as disabled ones', () => {
    expect(toRestrictedModules(['disabled-module', 'gentasks'], ['gentasks'], { gentasks: true })).toEqual(['disabled-module'])
  })

  it('treats a listed module without a switch as accessible', () => {
    expect(toRestrictedModules(['gentasks'], ['gentasks'], {})).toEqual([])
  })
})
