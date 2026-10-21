import { describe, expect, it } from 'vitest'
import { inviteFixture, rolesFixture } from '@/test/fixtures'
import { inviteActions, passwordsMatch, pickDefaultRole, type InviteStatus } from './invite'

const everyPermission = () => true
const noPermission = () => false

describe('inviteActions', () => {
  it.each<[InviteStatus, boolean, boolean]>([
    ['pending', true, true],
    ['expired', true, false],
    ['accepted', false, false],
    ['revoked', false, false],
  ])('for a %s invite allows resend=%s and revoke=%s', (status, canResend, canRevoke) => {
    expect(inviteActions(inviteFixture({ status }), everyPermission)).toEqual({ canResend, canRevoke })
  })

  it('requires the matching permission for each action', () => {
    const pending = inviteFixture({ status: 'pending' })
    expect(inviteActions(pending, noPermission)).toEqual({ canResend: false, canRevoke: false })
    expect(inviteActions(pending, (permission) => permission === 'core.invite.create')).toEqual({ canResend: true, canRevoke: false })
    expect(inviteActions(pending, (permission) => permission === 'core.invite.revoke')).toEqual({ canResend: false, canRevoke: true })
  })
})

describe('pickDefaultRole', () => {
  it('prefers the member role', () => {
    expect(pickDefaultRole(rolesFixture)?.name).toBe('member')
  })

  it('falls back to the first role, or nothing when there are none', () => {
    expect(pickDefaultRole([{ token: 'role-x', name: 'auditor', permissions: [] }])?.name).toBe('auditor')
    expect(pickDefaultRole([])).toBeUndefined()
  })
})

describe('passwordsMatch', () => {
  it('compares password and confirmation exactly', () => {
    expect(passwordsMatch('correct horse', 'correct horse')).toBe(true)
    expect(passwordsMatch('correct horse', 'correct Horse')).toBe(false)
    expect(passwordsMatch('correct horse', 'correct horse ')).toBe(false)
  })
})
