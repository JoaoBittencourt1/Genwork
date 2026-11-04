import { HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { setAccessToken } from '@/shared/datasource/httpClient'
import { captureRequests, errorResponse } from '@/test/server'
import { confirmPasswordReset, requestPasswordReset } from './passwordResetDatasource'

describe('passwordResetDatasource', () => {
  it('requests a reset by e-mail without credentials', async () => {
    const requests = captureRequests('post', '*/api/core/v1/auth/password-reset', () => new HttpResponse(null, { status: 204 }))
    setAccessToken('token-1')

    await expect(requestPasswordReset('ana@example.com')).resolves.toBeUndefined()
    expect(requests[0]?.body).toEqual({ email: 'ana@example.com' })
    expect(requests[0]?.headers.get('Authorization')).toBeNull()
  })

  it('confirms with the new password, encoding the secret in the path', async () => {
    const requests = captureRequests('post', '*/api/core/v1/auth/password-reset/:secret', () => new HttpResponse(null, { status: 204 }))

    await expect(confirmPasswordReset('se/cret', 'correct horse')).resolves.toBeUndefined()
    expect(requests[0]?.path).toBe('/api/core/v1/auth/password-reset/se%2Fcret')
    expect(requests[0]?.body).toEqual({ password: 'correct horse' })
    expect(requests[0]?.headers.get('Authorization')).toBeNull()
  })

  it('surfaces an unknown or expired link', async () => {
    captureRequests('post', '*/api/core/v1/auth/password-reset/:secret', () => errorResponse(404, 'password_reset_not_found'))
    await expect(confirmPasswordReset('gone', 'correct horse')).rejects.toMatchObject({ code: 'password_reset_not_found' })
  })
})
