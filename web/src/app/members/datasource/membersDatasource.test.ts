import { HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { setAccessToken } from '@/shared/datasource/httpClient'
import { memberFixture } from '@/test/memberFixtures'
import { captureRequests, errorResponse } from '@/test/server'
import { listMembers, removeMember, updateMember } from './membersDatasource'

describe('membersDatasource', () => {
  it('lists members from their envelope', async () => {
    captureRequests('get', '*/api/core/v1/members', () => HttpResponse.json({ members: [memberFixture()] }))
    setAccessToken('token-1')

    await expect(listMembers()).resolves.toEqual([memberFixture()])
  })

  it('patches only the changed field, encoding the member token in the path', async () => {
    const deactivated = memberFixture({ status: 'deactivated' })
    const requests = captureRequests('patch', '*/api/core/v1/members/:token', () => HttpResponse.json(deactivated))
    setAccessToken('token-1')

    await expect(updateMember('user/bruno', { status: 'deactivated' })).resolves.toEqual(deactivated)
    expect(requests[0]?.path).toBe('/api/core/v1/members/user%2Fbruno')
    expect(requests[0]?.body).toEqual({ status: 'deactivated' })
  })

  it('sends an empty module list to lift every restriction', async () => {
    const requests = captureRequests('patch', '*/api/core/v1/members/:token', () => HttpResponse.json(memberFixture()))

    await updateMember('user-bruno', { restrictedModules: [] })

    expect(requests[0]?.body).toEqual({ restrictedModules: [] })
  })

  it('removes a member by token', async () => {
    const requests = captureRequests('delete', '*/api/core/v1/members/:token', () => new HttpResponse(null, { status: 204 }))

    await expect(removeMember('user-bruno')).resolves.toBeUndefined()
    expect(requests[0]?.method).toBe('DELETE')
  })

  it.each([
    [403, 'cannot_manage_self'],
    [403, 'member_not_manageable'],
    [403, 'role_not_assignable'],
    [404, 'member_not_found'],
  ])('surfaces %s %s', async (status, code) => {
    captureRequests('patch', '*/api/core/v1/members/:token', () => errorResponse(status, code))
    captureRequests('delete', '*/api/core/v1/members/:token', () => errorResponse(status, code))

    await expect(updateMember('user-bruno', { role: 'role-admin' })).rejects.toMatchObject({ status, code })
    await expect(removeMember('user-bruno')).rejects.toMatchObject({ status, code })
  })
})
