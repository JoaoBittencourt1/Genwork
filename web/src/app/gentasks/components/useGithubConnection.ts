import { useQuery } from '@tanstack/react-query'
import { readGithubConnection } from '../datasource/githubDatasource'
import { gentasksQueryKeys } from './queryKeys'

export function useGithubConnection(enabled = true) {
  return useQuery({ queryKey: gentasksQueryKeys.githubConnection, queryFn: readGithubConnection, enabled })
}
