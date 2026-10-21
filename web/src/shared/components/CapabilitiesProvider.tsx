import { useQuery } from '@tanstack/react-query'
import { useMemo, type ReactNode } from 'react'
import { fetchCapabilities } from '@/shared/datasource/capabilities'
import { loadEnabledModules, NO_ENABLED_MODULES } from '@/shared/domain/capabilities'
import { CapabilitiesContext, type CapabilitiesContextValue } from './CapabilitiesContext'

// CapabilitiesProvider is mounted only for signed-in screens: the capabilities endpoint needs an access token.
export function CapabilitiesProvider({ children }: { children: ReactNode }) {
  const capabilities = useQuery({ queryKey: ['capabilities'], queryFn: () => loadEnabledModules({ fetchCapabilities }) })
  const value = useMemo<CapabilitiesContextValue>(
    () => ({ enabledModules: capabilities.data ?? NO_ENABLED_MODULES, isLoading: capabilities.isPending }),
    [capabilities.data, capabilities.isPending],
  )
  return <CapabilitiesContext.Provider value={value}>{children}</CapabilitiesContext.Provider>
}
