import { describe, expect, it } from 'vitest'
import { ApiError } from '@/shared/datasource/httpClient'
import { githubConnectionErrorKey, readManifestCallback } from './githubConnection'

describe('github connection', () => {
  it('reads the code and state GitHub sends back', () => {
    expect(readManifestCallback('?code=one-time-code&state=signed.state')).toEqual({ code: 'one-time-code', state: 'signed.state' })
  })

  it('rejects callbacks without a code or state', () => {
    expect(readManifestCallback('')).toBeNull()
    expect(readManifestCallback('?state=signed')).toBeNull()
    expect(readManifestCallback('?code=one-time-code')).toBeNull()
    expect(readManifestCallback('?code=&state=signed')).toBeNull()
  })

  it('explains why connecting failed', () => {
    expect(githubConnectionErrorKey(new ApiError(400, 'validation_failed', { state: 'invalid' }))).toBe('gentasks.github.errors.stateInvalid')
    expect(githubConnectionErrorKey(new ApiError(400, 'validation_failed', { code: 'invalid' }))).toBe('gentasks.github.errors.codeInvalid')
    expect(githubConnectionErrorKey(new ApiError(409, 'github_already_connected'))).toBe('errors.github_already_connected')
    expect(githubConnectionErrorKey(new ApiError(502, 'github_unavailable'))).toBe('errors.github_unavailable')
    expect(githubConnectionErrorKey(new Error('boom'))).toBe('errors.unknown_error')
  })
})
