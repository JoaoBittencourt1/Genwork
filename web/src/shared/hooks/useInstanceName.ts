import { useQuery } from '@tanstack/react-query'
import { fetchDiscovery } from '@/shared/datasource/discovery'

export function useInstanceName(): string | undefined {
  return useQuery({ queryKey: ['discovery'], queryFn: fetchDiscovery, staleTime: 5 * 60 * 1000 }).data?.name
}
