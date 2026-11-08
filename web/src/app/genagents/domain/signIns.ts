import type { Account, AccountSignIn, SignInState } from './types'

const ACTIVE_SIGN_IN_STATES: readonly SignInState[] = ['requested', 'awaiting_user', 'code_submitted', 'verifying']

// SIGN_IN_POLL_MS is how often the accounts list refreshes while a sign-in waits on the runner or the person.
export const SIGN_IN_POLL_MS = 2000

// SignInStep is what the account row shows for its latest sign-in attempt.
export type SignInStep = 'idle' | 'waiting_for_runner' | 'awaiting_user' | 'verifying' | 'ended'

export function isSignInActive(signIn: AccountSignIn | null): boolean {
  return signIn !== null && ACTIVE_SIGN_IN_STATES.includes(signIn.state)
}

// supportsWebSignIn says whether the account signs in through its provider CLI on a runner, from the web app.
export function supportsWebSignIn(account: Account): boolean {
  return account.mode === 'subscription' && account.runner !== null
}

// signInStep turns the latest attempt into what the row shows; a finished success shows nothing more than the status.
export function signInStep(account: Account): SignInStep {
  const signIn = account.signIn
  if (!supportsWebSignIn(account) || signIn === null) return 'idle'
  switch (signIn.state) {
    case 'requested':
      return 'waiting_for_runner'
    case 'awaiting_user':
      return 'awaiting_user'
    case 'code_submitted':
    case 'verifying':
      return 'verifying'
    case 'succeeded':
      return 'idle'
    default:
      return 'ended'
  }
}

// signInButtonLabelKey picks "Log in with Claude" for a first sign-in and "Sign in again" when the account was signed in
// before or its last attempt ended.
export function signInButtonLabelKey(account: Account): 'genagents.accounts.signIn.start' | 'genagents.accounts.signIn.again' {
  return account.status === 'pending_sign_in' && (account.signIn === null || account.signIn.state === 'requested')
    ? 'genagents.accounts.signIn.start'
    : 'genagents.accounts.signIn.again'
}

// canStartSignIn hides the button while an attempt is in flight; the row then shows that attempt's step instead.
export function canStartSignIn(account: Account): boolean {
  return supportsWebSignIn(account) && !isSignInActive(account.signIn)
}

// shouldPollAccounts keeps the list fresh while any sign-in is in flight.
export function shouldPollAccounts(accounts: Account[] | undefined): boolean {
  return (accounts ?? []).some((account) => isSignInActive(account.signIn))
}

// canSubmitSignInCode accepts what provider pages show: printable characters without spaces.
export function canSubmitSignInCode(code: string): boolean {
  const trimmed = code.trim()
  return trimmed.length > 0 && trimmed.length <= 512 && !/\s/.test(trimmed)
}

const KNOWN_SIGN_IN_ERRORS = ['sign_in_expired'] as const

// signInErrorKey is the translation key of a failure genagents records as a code; other failures are the CLI's own text.
export function signInErrorKey(message: string | null): string | null {
  return message !== null && (KNOWN_SIGN_IN_ERRORS as readonly string[]).includes(message) ? `genagents.accounts.signIn.errors.${message}` : null
}
