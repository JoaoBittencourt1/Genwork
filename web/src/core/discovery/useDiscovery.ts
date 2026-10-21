import { useQuery } from '@tanstack/react-query'
import { apiRequest } from '../../shared/api/client'

export type DiscoveryDocument = {
  product: string
  name: string
  version: string
  api: string
  auth: { methods: string[] }
}

export function useDiscovery() {
  return useQuery({
    queryKey: ['discovery'],
    queryFn: () => apiRequest<DiscoveryDocument>('GET', '/.well-known/genwork', { authenticated: false }),
    staleTime: 5 * 60 * 1000,
  })
}

export function useInstanceName(): string | undefined {
  return useDiscovery().data?.name
}
