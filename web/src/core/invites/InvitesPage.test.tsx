import { screen, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { adminFixture, deliveryFixture, inviteFixture, memberFixture, rolesFixture } from '../../test/fixtures'
import { renderApp } from '../../test/renderApp'
import { errorResponse, server, useSignedInSession } from '../../test/server'

function useInviteEndpoints(initialInvites = [inviteFixture()]) {
  let invites = initialInvites
  const calls = { created: [] as unknown[], resent: [] as string[], revoked: [] as string[] }
  server.use(
    http.get('*/api/core/v1/invites', () => HttpResponse.json({ invites })),
    http.get('*/api/core/v1/roles', () => HttpResponse.json({ roles: rolesFixture })),
    http.post('*/api/core/v1/invites', async ({ request }) => {
      const body = (await request.json()) as { email: string; role: string }
      calls.created.push(body)
      const delivery = deliveryFixture({
        invite: inviteFixture({ token: 'invite-dora', email: body.email }),
        link: 'http://localhost:8485/invite/secret-dora',
        emailSent: false,
      })
      invites = [delivery.invite, ...invites]
      return HttpResponse.json(delivery, { status: 201 })
    }),
    http.post('*/api/core/v1/invites/:token/resend', ({ params }) => {
      calls.resent.push(String(params.token))
      return HttpResponse.json(deliveryFixture({ link: 'http://localhost:8485/invite/secret-new' }))
    }),
    http.delete('*/api/core/v1/invites/:token', ({ params }) => {
      calls.revoked.push(String(params.token))
      invites = invites.map((invite) => (invite.token === params.token ? { ...invite, status: 'revoked' as const } : invite))
      return new HttpResponse(null, { status: 204 })
    }),
  )
  return calls
}

describe('InvitesPage', () => {
  it('invites someone and shows the link to copy', async () => {
    useSignedInSession(adminFixture)
    const calls = useInviteEndpoints()
    const { user } = renderApp('/invites')

    await user.type(await screen.findByLabelText('E-mail'), 'dora@example.com')
    expect(await screen.findByRole('option', { name: 'member' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Convidar' }))

    const linkField = await screen.findByLabelText('Link do convite')
    expect(linkField).toHaveValue('http://localhost:8485/invite/secret-dora')
    expect(screen.getByText('Não foi possível enviar o e-mail. Copie o link e envie manualmente.')).toBeInTheDocument()
    expect(calls.created).toEqual([{ email: 'dora@example.com', role: 'role-member' }])

    await user.click(screen.getByRole('button', { name: 'Copiar link' }))
    expect(await navigator.clipboard.readText()).toBe('http://localhost:8485/invite/secret-dora')
    expect(screen.getByRole('button', { name: 'Link copiado' })).toBeInTheDocument()
    expect(await screen.findByRole('cell', { name: 'dora@example.com' })).toBeInTheDocument()
  })

  it('translates invite errors', async () => {
    useSignedInSession(adminFixture)
    useInviteEndpoints()
    server.use(http.post('*/api/core/v1/invites', () => errorResponse(409, 'invite_already_pending')))
    const { user } = renderApp('/invites')

    await user.type(await screen.findByLabelText('E-mail'), 'carla@example.com')
    await screen.findByRole('option', { name: 'member' })
    await user.click(screen.getByRole('button', { name: 'Convidar' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Este e-mail já tem um convite pendente.')
  })

  it('resends and revokes from the list', async () => {
    useSignedInSession(adminFixture)
    const calls = useInviteEndpoints()
    const { user } = renderApp('/invites')

    const row = (await screen.findByRole('cell', { name: 'carla@example.com' })).closest('tr') as HTMLElement
    expect(within(row).getByText('Pendente')).toBeInTheDocument()

    await user.click(within(row).getByRole('button', { name: 'Reenviar' }))
    expect(await screen.findByLabelText('Link do convite')).toHaveValue('http://localhost:8485/invite/secret-new')
    expect(calls.resent).toEqual(['invite-carla'])

    await user.click(within(row).getByRole('button', { name: 'Revogar' }))
    expect(await within(row).findByText('Revogado')).toBeInTheDocument()
    expect(calls.revoked).toEqual(['invite-carla'])
    expect(within(row).queryByRole('button', { name: 'Revogar' })).not.toBeInTheDocument()
  })

  it('shows the Invites entry only with permission', async () => {
    useSignedInSession(adminFixture)
    useInviteEndpoints()
    renderApp('/')
    expect(await screen.findByRole('link', { name: 'Convites' })).toBeInTheDocument()
  })

  it('hides invites from members', async () => {
    useSignedInSession(memberFixture)
    const first = renderApp('/')
    expect(await screen.findByRole('heading', { name: 'Olá, Bruno!' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Convites' })).not.toBeInTheDocument()
    first.unmount()

    renderApp('/invites')
    expect(await screen.findByRole('heading', { name: 'Sem permissão' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Convidar' })).not.toBeInTheDocument()
  })
})
