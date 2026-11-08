import { describe, expect, it } from 'vitest'
import { accountFixture, signInFixture, subscriptionAccountFixture } from '@/test/genagentsFixtures'
import { accountSignInHint } from './providers'
import {
  canStartSignIn,
  canSubmitSignInCode,
  isSignInActive,
  shouldPollAccounts,
  signInButtonLabelKey,
  signInErrorKey,
  signInStep,
  supportsWebSignIn,
} from './signIns'
import type { SignInState } from './types'

describe('web sign-in of subscription accounts', () => {
  it('is offered only to subscription accounts held by a runner, never to API-key accounts', () => {
    expect(supportsWebSignIn(subscriptionAccountFixture())).toBe(true)
    expect(supportsWebSignIn(accountFixture())).toBe(false)
    expect(supportsWebSignIn(subscriptionAccountFixture({ runner: null }))).toBe(false)
    expect(canStartSignIn(accountFixture())).toBe(false)
  })

  it('shows the login button until an attempt is in flight, then that attempt instead', () => {
    expect(canStartSignIn(subscriptionAccountFixture())).toBe(true)
    for (const state of ['requested', 'awaiting_user', 'code_submitted', 'verifying'] as const) {
      expect(canStartSignIn(subscriptionAccountFixture({ signIn: signInFixture({ state }) }))).toBe(false)
    }
    for (const state of ['succeeded', 'failed', 'cancelled', 'expired'] as const) {
      expect(canStartSignIn(subscriptionAccountFixture({ signIn: signInFixture({ state }) }))).toBe(true)
    }
  })

  it('maps each attempt state to the step the row shows', () => {
    const stepOf = (state: SignInState) => signInStep(subscriptionAccountFixture({ signIn: signInFixture({ state }) }))
    expect(signInStep(subscriptionAccountFixture())).toBe('idle')
    expect(stepOf('requested')).toBe('waiting_for_runner')
    expect(stepOf('awaiting_user')).toBe('awaiting_user')
    expect(stepOf('code_submitted')).toBe('verifying')
    expect(stepOf('verifying')).toBe('verifying')
    expect(stepOf('succeeded')).toBe('idle')
    expect(stepOf('failed')).toBe('ended')
    expect(stepOf('cancelled')).toBe('ended')
    expect(stepOf('expired')).toBe('ended')
    expect(signInStep(accountFixture({ signIn: signInFixture({ state: 'awaiting_user' }) }))).toBe('idle')
  })

  it('labels a first sign-in "log in" and any later one "sign in again"', () => {
    expect(signInButtonLabelKey(subscriptionAccountFixture())).toBe('genagents.accounts.signIn.start')
    expect(signInButtonLabelKey(subscriptionAccountFixture({ status: 'ready' }))).toBe('genagents.accounts.signIn.again')
    expect(signInButtonLabelKey(subscriptionAccountFixture({ status: 'error', signIn: signInFixture({ state: 'expired' }) }))).toBe(
      'genagents.accounts.signIn.again',
    )
  })

  it('polls the accounts only while some sign-in is in flight', () => {
    expect(shouldPollAccounts(undefined)).toBe(false)
    expect(shouldPollAccounts([subscriptionAccountFixture(), accountFixture()])).toBe(false)
    expect(shouldPollAccounts([subscriptionAccountFixture({ signIn: signInFixture({ state: 'verifying' }) })])).toBe(true)
    expect(isSignInActive(null)).toBe(false)
  })

  it('accepts a pasted code without spaces, trimming the edges', () => {
    expect(canSubmitSignInCode('  abc#def  ')).toBe(true)
    expect(canSubmitSignInCode('')).toBe(false)
    expect(canSubmitSignInCode('has space')).toBe(false)
    expect(canSubmitSignInCode('x'.repeat(513))).toBe(false)
  })

  it('translates the failures genagents records as codes and leaves CLI messages as they are', () => {
    expect(signInErrorKey('sign_in_expired')).toBe('genagents.accounts.signIn.errors.sign_in_expired')
    expect(signInErrorKey('OAuth error: Invalid code')).toBeNull()
    expect(signInErrorKey(null)).toBeNull()
  })

  it('keeps the runner command as the fallback while a subscription account is not ready', () => {
    expect(accountSignInHint(subscriptionAccountFixture())).toBe('genwork-runner account login account-1')
    expect(accountSignInHint(subscriptionAccountFixture({ status: 'error' }))).toBe('genwork-runner account login account-1')
    expect(accountSignInHint(subscriptionAccountFixture({ status: 'ready' }))).toBeNull()
    expect(accountSignInHint(accountFixture({ status: 'error', signInCommand: 'x' }))).toBeNull()
  })
})
