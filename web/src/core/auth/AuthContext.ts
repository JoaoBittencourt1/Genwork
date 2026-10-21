import { createContext, useContext } from 'react'
import type { Me } from './api'

export type AuthState = { status: 'loading' } | { status: 'signedOut' } | { status: 'signedIn'; me: Me }

export type AuthContextValue = {
  state: AuthState
  signIn: (email: string, password: string) => Promise<void>
  completeSignIn: (accessToken: string) => Promise<void>
  signOut: () => Promise<void>
  can: (permission: string) => boolean
}

export const AuthContext = createContext<AuthContextValue | null>(null)

export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext)
  if (!value) {
    throw new Error('useAuth must be used inside AuthProvider')
  }
  return value
}

export const PERMISSION_INVITE_CREATE = 'core.invite.create'
export const PERMISSION_INVITE_READ = 'core.invite.read'
export const PERMISSION_INVITE_REVOKE = 'core.invite.revoke'
