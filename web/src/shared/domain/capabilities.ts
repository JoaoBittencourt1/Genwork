import type { Capabilities } from '@/shared/datasource/capabilities'

// Module ids as their module.yaml declares them; clients never assume a module is enabled (rule 58).
export const MODULE_IDS = { gentasks: 'gentasks', genmonitor: 'genmonitor' } as const
export type ModuleId = (typeof MODULE_IDS)[keyof typeof MODULE_IDS]

export type EnabledModules = ReadonlySet<string>

export const NO_ENABLED_MODULES: EnabledModules = new Set()

export type CapabilitiesGateway = { fetchCapabilities: () => Promise<Capabilities> }

// loadEnabledModules treats a failed request as "no module enabled", so core screens keep working.
export async function loadEnabledModules(gateway: CapabilitiesGateway): Promise<EnabledModules> {
  try {
    const capabilities = await gateway.fetchCapabilities()
    return new Set(capabilities.modules.map((module) => module.id))
  } catch {
    return NO_ENABLED_MODULES
  }
}

export function isModuleEnabled(enabled: EnabledModules, moduleId: ModuleId): boolean {
  return enabled.has(moduleId)
}

// Callers pass the module id as defaultValue, so a module without copy still shows a readable name.
export function moduleNameTranslationKey(moduleId: string): string {
  return `modules.names.${moduleId}`
}
