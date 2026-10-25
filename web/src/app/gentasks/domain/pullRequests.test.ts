import { describe, expect, it } from 'vitest'
import { pullRequestFixture } from '@/test/gentasksFixtures'
import { pullRequestReference, pullRequestStateKey, pullRequestTitle, sortPullRequests } from './pullRequests'

describe('pull requests', () => {
  it('orders open, draft, unknown, merged, then closed, newest number first within a state', () => {
    const pullRequests = [
      pullRequestFixture({ number: 31, state: 'closed' }),
      pullRequestFixture({ number: 32, state: 'merged' }),
      pullRequestFixture({ number: 3, state: null }),
      pullRequestFixture({ number: 34, state: 'open' }),
      pullRequestFixture({ number: 33, state: 'draft' }),
      pullRequestFixture({ number: 35, state: 'open' }),
    ]

    expect(sortPullRequests(pullRequests).map((pullRequest) => pullRequest.number)).toEqual([35, 34, 33, 3, 32, 31])
    expect(pullRequests[0]?.number).toBe(31)
  })

  it('references a pull request by repository and number', () => {
    expect(pullRequestReference(pullRequestFixture({ repository: 'acme/web', number: 4 }))).toBe('acme/web#4')
  })

  it('falls back to the reference when the title is unknown', () => {
    expect(pullRequestTitle(pullRequestFixture({ title: 'Sign in' }))).toBe('Sign in')
    expect(pullRequestTitle(pullRequestFixture({ title: null, number: 3 }))).toBe('acme/api#3')
  })

  it('maps an unknown state to the unknown key', () => {
    expect(pullRequestStateKey(pullRequestFixture({ state: 'merged' }))).toBe('merged')
    expect(pullRequestStateKey(pullRequestFixture({ state: null }))).toBe('unknown')
  })
})
