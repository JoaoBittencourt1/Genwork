import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { onSessionEnded, setAccessToken } from '@/shared/datasource/httpClient'
import { fetchMe, login, logout, sessionGateway } from '../datasource/authDatasource'
import { hasPermission, restoreSession, type AuthState } from '../domain/session'
import { AuthContext, type AuthContextValue } from './AuthContext'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: 'loading' })

  const completeSignIn = useCallback(async (accessToken: string) => {
    setAccessToken(accessToken)
    setState({ status: 'signedIn', me: await fetchMe() })
  }, [])

  useEffect(() => {
    let cancelled = false
    const stopListening = onSessionEnded(() => setState({ status: 'signedOut' }))
    restoreSession(sessionGateway).then((restored) => {
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
    () => ({ state, signIn, completeSignIn, signOut, can: (permission) => hasPermission(state, permission) }),
    [state, signIn, completeSignIn, signOut],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
