import { createContext, useContext } from 'react'
import { NO_ENABLED_MODULES, type EnabledModules } from '@/shared/domain/capabilities'

export type CapabilitiesContextValue = {
  enabledModules: EnabledModules
  isLoading: boolean
}

export const CapabilitiesContext = createContext<CapabilitiesContextValue>({ enabledModules: NO_ENABLED_MODULES, isLoading: false })

export function useCapabilities(): CapabilitiesContextValue {
  return useContext(CapabilitiesContext)
}
