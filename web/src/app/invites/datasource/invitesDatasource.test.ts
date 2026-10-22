import { HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { setAccessToken } from '@/shared/datasource/httpClient'
import { deliveryFixture, inviteFixture, signInFixture } from '@/test/fixtures'
import { captureRequests, errorResponse } from '@/test/server'
import {
  acceptInvitation,
  createInvite,
  inspectInvitation,
  listInvites,
  resendInvite,
  revokeInvite,
} from './invitesDatasource'

describe('invitesDatasource', () => {
  it('lists invites from their envelope', async () => {
    captureRequests('get', '*/api/core/v1/invites', () => HttpResponse.json({ invites: [inviteFixture()] }))
    setAccessToken('token-1')

    await expect(listInvites()).resolves.toEqual([inviteFixture()])
  })

  it('creates an invite with e-mail and role token', async () => {
    const requests = captureRequests('post', '*/api/core/v1/invites', () => HttpResponse.json(deliveryFixture(), { status: 201 }))
    setAccessToken('token-1')

    await expect(createInvite('carla@example.com', 'role-member')).resolves.toEqual(deliveryFixture())
    expect(requests[0]?.body).toEqual({ email: 'carla@example.com', role: 'role-member' })
    expect(requests[0]?.headers.get('Authorization')).toBe('Bearer token-1')
  })

  it('surfaces conflicts when creating invites', async () => {
    captureRequests('post', '*/api/core/v1/invites', () => errorResponse(409, 'invite_already_pending'))
    await expect(createInvite('carla@example.com', 'role-member')).rejects.toMatchObject({ code: 'invite_already_pending' })
  })

  it('resends and revokes by invite token, encoding it in the path', async () => {
    const resent = captureRequests('post', '*/api/core/v1/invites/:token/resend', () => HttpResponse.json(deliveryFixture()))
    const revoked = captureRequests('delete', '*/api/core/v1/invites/:token', () => new HttpResponse(null, { status: 204 }))

    await expect(resendInvite('invite/carla')).resolves.toEqual(deliveryFixture())
    await expect(revokeInvite('invite/carla')).resolves.toBeUndefined()
    expect(resent[0]?.path).toBe('/api/core/v1/invites/invite%2Fcarla/resend')
    expect(revoked[0]?.path).toBe('/api/core/v1/invites/invite%2Fcarla')
  })

  it('inspects and accepts invitations without credentials', async () => {
    const invitation = { instanceName: 'Genwork', email: 'carla@example.com', expiresAt: '2026-10-27T12:00:00Z' }
    const inspected = captureRequests('get', '*/api/core/v1/invitations/:secret', () => HttpResponse.json(invitation))
    const accepted = captureRequests('post', '*/api/core/v1/invitations/:secret/accept', () => HttpResponse.json(signInFixture))
    setAccessToken('token-1')

    await expect(inspectInvitation('secret-carla')).resolves.toEqual(invitation)
    await expect(acceptInvitation('secret-carla', 'Carla', 'correct horse')).resolves.toEqual(signInFixture)
    expect(inspected[0]?.headers.get('Authorization')).toBeNull()
    expect(accepted[0]?.headers.get('Authorization')).toBeNull()
    expect(accepted[0]?.body).toEqual({ name: 'Carla', password: 'correct horse', client: 'web' })
  })
})
