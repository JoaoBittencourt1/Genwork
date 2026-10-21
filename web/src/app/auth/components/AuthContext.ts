import { createContext, useContext } from 'react'
import type { AuthState } from '../domain/session'

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
