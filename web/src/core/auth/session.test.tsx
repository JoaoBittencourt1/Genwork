import { screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { adminFixture } from '../../test/fixtures'
import { renderApp } from '../../test/renderApp'
import { server, useSignedInSession } from '../../test/server'

describe('session lifecycle', () => {
  it('restores the session on page load', async () => {
    useSignedInSession(adminFixture)
    renderApp('/')
    expect(await screen.findByRole('heading', { name: 'Olá, Ana!' })).toBeInTheDocument()
  })

  it('signs out and returns to the login screen', async () => {
    useSignedInSession(adminFixture)
    let loggedOut = false
    server.use(
      http.post('*/api/core/v1/auth/logout', () => {
        loggedOut = true
        return new HttpResponse(null, { status: 204 })
      }),
    )
    const { user } = renderApp('/')

    await user.click(await screen.findByRole('button', { name: 'Sair' }))

    expect(await screen.findByRole('heading', { name: 'Entrar' })).toBeInTheDocument()
    expect(loggedOut).toBe(true)
  })
})
