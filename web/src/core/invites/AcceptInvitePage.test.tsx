import { screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { invitationFixture, memberFixture } from '../../test/fixtures'
import { renderApp } from '../../test/renderApp'
import { errorResponse, server } from '../../test/server'

const path = '/invite/secret-carla'

describe('AcceptInvitePage', () => {
  it('creates the account and lands on home', async () => {
    let acceptBody: unknown
    server.use(
      http.get('*/api/core/v1/invitations/secret-carla', () => HttpResponse.json(invitationFixture)),
      http.post('*/api/core/v1/invitations/secret-carla/accept', async ({ request }) => {
        acceptBody = await request.json()
        return HttpResponse.json({ accessToken: 'access-token', accessTokenExpiresAt: '2026-10-20T12:15:00Z', session: 's' })
      }),
      http.get('*/api/core/v1/me', () => HttpResponse.json({ ...memberFixture, name: 'Carla', email: 'carla@example.com' })),
    )
    const { user } = renderApp(path)

    expect(await screen.findByText('Crie sua conta para entrar em Genwork da Empresa.')).toBeInTheDocument()
    expect(screen.getByLabelText('E-mail')).toHaveValue('carla@example.com')
    expect(screen.getByLabelText('E-mail')).toHaveAttribute('readonly')

    await user.type(screen.getByLabelText('Nome'), 'Carla')
    await user.type(screen.getByLabelText('Senha'), 'correct horse battery')
    await user.type(screen.getByLabelText('Confirme a senha'), 'correct horse battery')
    await user.click(screen.getByRole('button', { name: 'Criar conta e entrar' }))

    expect(await screen.findByRole('heading', { name: 'Olá, Carla!' })).toBeInTheDocument()
    expect(acceptBody).toEqual({ name: 'Carla', password: 'correct horse battery', client: 'web' })
  })

  it('blocks mismatched passwords without calling the API', async () => {
    let acceptCalls = 0
    server.use(
      http.get('*/api/core/v1/invitations/secret-carla', () => HttpResponse.json(invitationFixture)),
      http.post('*/api/core/v1/invitations/secret-carla/accept', () => {
        acceptCalls += 1
        return errorResponse(500, 'internal_error')
      }),
    )
    const { user } = renderApp(path)

    await user.type(await screen.findByLabelText('Nome'), 'Carla')
    await user.type(screen.getByLabelText('Senha'), 'correct horse battery')
    await user.type(screen.getByLabelText('Confirme a senha'), 'different horse battery')
    await user.click(screen.getByRole('button', { name: 'Criar conta e entrar' }))

    expect(await screen.findByText('As senhas não conferem.')).toBeInTheDocument()
    expect(acceptCalls).toBe(0)
  })

  it('shows the server password rule', async () => {
    server.use(
      http.get('*/api/core/v1/invitations/secret-carla', () => HttpResponse.json(invitationFixture)),
      http.post('*/api/core/v1/invitations/secret-carla/accept', () => errorResponse(400, 'password_length_invalid')),
    )
    const { user } = renderApp(path)

    await user.type(await screen.findByLabelText('Nome'), 'Carla')
    await user.type(screen.getByLabelText('Senha'), 'short')
    await user.type(screen.getByLabelText('Confirme a senha'), 'short')
    await user.click(screen.getByRole('button', { name: 'Criar conta e entrar' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('A senha precisa ter entre 8 e 128 caracteres.')
  })

  it('shows a not-found screen for invalid links', async () => {
    server.use(http.get('*/api/core/v1/invitations/secret-carla', () => errorResponse(404, 'invite_not_found')))
    renderApp(path)
    expect(await screen.findByRole('heading', { name: 'Convite não encontrado' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Ir para o login' })).toBeInTheDocument()
  })
})
