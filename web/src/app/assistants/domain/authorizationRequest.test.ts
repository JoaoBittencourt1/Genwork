import { describe, expect, it } from 'vitest'
import { claudeAuthorizationQuery, claudeAuthorizationRequest } from '@/test/assistantFixtures'
import { parseAuthorizationRequest, toAuthorizationQuery } from './authorizationRequest'

describe('authorization request', () => {
  it('reads every parameter Claude sends', () => {
    expect(parseAuthorizationRequest(claudeAuthorizationQuery)).toEqual(claudeAuthorizationRequest)
  })

  it('reads missing parameters as empty, leaving the validation to core', () => {
    expect(parseAuthorizationRequest('?client_id=abc')).toEqual({
      responseType: '',
      clientId: 'abc',
      redirectUri: '',
      state: '',
      codeChallenge: '',
      codeChallengeMethod: '',
      scope: '',
      resource: '',
    })
  })

  it('has nothing to ask core about without a client', () => {
    expect(parseAuthorizationRequest('')).toBeNull()
    expect(parseAuthorizationRequest('?state=x')).toBeNull()
  })

  it('writes the request back as the OAuth query', () => {
    expect(new URLSearchParams(toAuthorizationQuery(claudeAuthorizationRequest))).toEqual(new URLSearchParams(claudeAuthorizationQuery))
  })
})
