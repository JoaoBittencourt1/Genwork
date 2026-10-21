import { apiRequest } from './httpClient'

export type DiscoveryDocument = {
  product: string
  name: string
  version: string
  api: string
  auth: { methods: string[] }
}

export function fetchDiscovery(): Promise<DiscoveryDocument> {
  return apiRequest<DiscoveryDocument>('GET', '/.well-known/genwork', { authenticated: false })
}
