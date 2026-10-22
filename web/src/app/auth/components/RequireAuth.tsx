import { Navigate, useLocation } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { LoadingScreen } from '@/shared/components/LoadingScreen'
import { loginRedirectSearch } from '../domain/safeRedirectTarget'
import { useAuth } from './AuthContext'

export function RequireAuth({ children }: { children: ReactNode }) {
  const { state } = useAuth()
  const location = useLocation()
  if (state.status === 'loading') return <LoadingScreen />
  if (state.status === 'signedOut') return <Navigate to="/login" search={loginRedirectSearch(location.href)} />
  return <>{children}</>
}
