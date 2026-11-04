import { useQuery } from '@tanstack/react-query'
import { fetchDiscovery } from '@/shared/datasource/discovery'
import { NO_LIVE_REFRESH } from '@/shared/domain/liveRefresh'

export function useInstanceName(): string | undefined {
  return useQuery({ queryKey: ['discovery'], queryFn: fetchDiscovery, staleTime: 5 * 60 * 1000, ...NO_LIVE_REFRESH }).data?.name
}
