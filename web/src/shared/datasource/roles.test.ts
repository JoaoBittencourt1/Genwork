import { HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { memberRolesFixture } from '@/test/memberFixtures'
import { captureRequests } from '@/test/server'
import { setAccessToken } from './httpClient'
import { listRoles } from './roles'

describe('roles datasource', () => {
  it('lists roles from their envelope', async () => {
    captureRequests('get', '*/api/core/v1/roles', () => HttpResponse.json({ roles: memberRolesFixture }))
    setAccessToken('token-1')

    await expect(listRoles()).resolves.toEqual(memberRolesFixture)
  })
})
