import { describe, expect, it, vi } from 'vitest'
import { adminFixture, memberFixture } from '@/test/fixtures'
import { hasPermission, PERMISSIONS, restoreSession, type AuthState } from './session'

describe('hasPermission', () => {
  it('grants permissions from the signed-in user only', () => {
    const admin: AuthState = { status: 'signedIn', me: adminFixture }
    const member: AuthState = { status: 'signedIn', me: memberFixture }

    expect(hasPermission(admin, PERMISSIONS.inviteRead)).toBe(true)
    expect(hasPermission(member, PERMISSIONS.inviteRead)).toBe(false)
    expect(hasPermission({ status: 'signedOut' }, PERMISSIONS.inviteRead)).toBe(false)
    expect(hasPermission({ status: 'loading' }, PERMISSIONS.inviteRead)).toBe(false)
  })
})

describe('restoreSession', () => {
  it('signs in when the refresh succeeds and the profile loads', async () => {
    const state = await restoreSession({ refresh: async () => true, fetchMe: async () => adminFixture })
    expect(state).toEqual({ status: 'signedIn', me: adminFixture })
  })

  it('stays signed out without fetching the profile when refresh fails', async () => {
    const fetchMe = vi.fn(async () => adminFixture)
    const state = await restoreSession({ refresh: async () => false, fetchMe })
    expect(state).toEqual({ status: 'signedOut' })
    expect(fetchMe).not.toHaveBeenCalled()
  })

  it('stays signed out when the profile cannot be loaded', async () => {
    const state = await restoreSession({
      refresh: async () => true,
      fetchMe: async () => {
        throw new Error('network down')
      },
    })
    expect(state).toEqual({ status: 'signedOut' })
  })
})
