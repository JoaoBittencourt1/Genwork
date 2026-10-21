import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { onSessionEnded, refreshSession, setAccessToken } from '../../shared/api/client'
import { fetchMe, login, logout } from './api'
import { AuthContext, type AuthContextValue, type AuthState } from './AuthContext'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: 'loading' })

  const completeSignIn = useCallback(async (accessToken: string) => {
    setAccessToken(accessToken)
    const me = await fetchMe()
    setState({ status: 'signedIn', me })
  }, [])

  useEffect(() => {
    let cancelled = false
    const stopListening = onSessionEnded(() => setState({ status: 'signedOut' }))
    restoreSession().then((restored) => {
      if (!cancelled) setState(restored)
    })
    return () => {
      cancelled = true
      stopListening()
    }
  }, [])

  const signIn = useCallback(
    async (email: string, password: string) => {
      const response = await login(email, password)
      await completeSignIn(response.accessToken)
    },
    [completeSignIn],
  )

  const signOut = useCallback(async () => {
    try {
      await logout()
    } finally {
      setAccessToken(null)
      setState({ status: 'signedOut' })
    }
  }, [])

  const value = useMemo<AuthContextValue>(
    () => ({
      state,
      signIn,
      completeSignIn,
      signOut,
      can: (permission) => state.status === 'signedIn' && state.me.permissions.includes(permission),
    }),
    [state, signIn, completeSignIn, signOut],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

async function restoreSession(): Promise<AuthState> {
  if (!(await refreshSession())) {
    return { status: 'signedOut' }
  }
  try {
    return { status: 'signedIn', me: await fetchMe() }
  } catch {
    return { status: 'signedOut' }
  }
}
