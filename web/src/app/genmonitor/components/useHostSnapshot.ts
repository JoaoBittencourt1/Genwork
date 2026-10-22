import { useQuery } from '@tanstack/react-query'
import { readHostSnapshot } from '../datasource/hostDatasource'
import { refreshIntervalMs } from '../domain/hostSnapshot'
import { genmonitorQueryKeys } from './queryKeys'

export function useHostSnapshot(enabled: boolean) {
  return useQuery({
    queryKey: genmonitorQueryKeys.host,
    queryFn: readHostSnapshot,
    enabled,
    refetchInterval: (query) => refreshIntervalMs(query.state.data),
  })
}
