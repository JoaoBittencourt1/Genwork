import { apiRequest } from './httpClient'

export type ModuleCapability = { id: string; version: string; apiPrefix: string }

export type Capabilities = {
  core: { version: string; api: string }
  modules: ModuleCapability[]
}

export function fetchCapabilities(): Promise<Capabilities> {
  return apiRequest<Capabilities>('GET', '/api/core/v1/capabilities')
}
