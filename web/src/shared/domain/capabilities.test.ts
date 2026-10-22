import { describe, expect, it } from 'vitest'
import { capabilitiesFixture } from '@/test/gentasksFixtures'
import { isModuleEnabled, loadEnabledModules, MODULE_IDS, moduleNameTranslationKey } from './capabilities'

describe('capabilities', () => {
  it('enables the modules the capabilities list', async () => {
    const enabled = await loadEnabledModules({ fetchCapabilities: () => Promise.resolve(capabilitiesFixture) })
    expect(isModuleEnabled(enabled, MODULE_IDS.gentasks)).toBe(true)
  })

  it('enables nothing when no module is registered', async () => {
    const enabled = await loadEnabledModules({
      fetchCapabilities: () => Promise.resolve({ ...capabilitiesFixture, modules: [] }),
    })
    expect(isModuleEnabled(enabled, MODULE_IDS.gentasks)).toBe(false)
  })

  it('treats a failed request as every module disabled', async () => {
    const enabled = await loadEnabledModules({ fetchCapabilities: () => Promise.reject(new Error('down')) })
    expect(enabled.size).toBe(0)
  })
})

describe('moduleNameTranslationKey', () => {
  it('names a module through the modules.names namespace', () => {
    expect(moduleNameTranslationKey('gentasks')).toBe('modules.names.gentasks')
  })
})
