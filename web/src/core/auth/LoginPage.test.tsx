import { screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { adminFixture } from '../../test/fixtures'
import { renderApp } from '../../test/renderApp'
import { errorResponse, server } from '../../test/server'

describe('LoginPage', () => {
  it('shows the instance name from discovery', async () => {
    renderApp('/login')
    expect(await screen.findByText('Entre na sua conta de Genwork da Empresa')).toBeInTheDocument()
  })

  it('signs in and lands on the home screen', async () => {
    let loginBody: unknown
    server.use(
      http.post('*/api/core/v1/auth/login', async ({ request }) => {
        loginBody = await request.json()
        return HttpResponse.json({ accessToken: 'access-token', accessTokenExpiresAt: '2026-10-20T12:15:00Z', session: 's' })
      }),
      http.get('*/api/core/v1/me', () => HttpResponse.json(adminFixture)),
    )
    const { user } = renderApp('/login')

    await user.type(await screen.findByLabelText('E-mail'), 'ana@example.com')
    await user.type(screen.getByLabelText('Senha'), 'correct horse battery')
    await user.click(screen.getByRole('button', { name: 'Entrar' }))

    expect(await screen.findByRole('heading', { name: 'Olá, Ana!' })).toBeInTheDocument()
    expect(loginBody).toEqual({ email: 'ana@example.com', password: 'correct horse battery', client: 'web' })
    expect(screen.getByRole('button', { name: 'Sair' })).toBeInTheDocument()
  })

  it('shows one message for wrong e-mail or password', async () => {
    server.use(http.post('*/api/core/v1/auth/login', () => errorResponse(401, 'invalid_credentials')))
    const { user } = renderApp('/login')

    await user.type(await screen.findByLabelText('E-mail'), 'ana@example.com')
    await user.type(screen.getByLabelText('Senha'), 'wrong password!')
    await user.click(screen.getByRole('button', { name: 'Entrar' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('E-mail ou senha incorretos.')
  })

  it('redirects to login when there is no session', async () => {
    const { history } = renderApp('/')
    expect(await screen.findByRole('heading', { name: 'Entrar' })).toBeInTheDocument()
    expect(history.location.pathname).toBe('/login')
  })
})
