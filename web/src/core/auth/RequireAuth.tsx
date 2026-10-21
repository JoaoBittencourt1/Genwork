import { Navigate } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { LoadingScreen } from '../../shared/ui/LoadingScreen'
import { useAuth } from './AuthContext'

export function RequireAuth({ children }: { children: ReactNode }) {
  const { state } = useAuth()
  if (state.status === 'loading') return <LoadingScreen />
  if (state.status === 'signedOut') return <Navigate to="/login" />
  return <>{children}</>
}
