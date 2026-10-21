import { useQuery } from '@tanstack/react-query'
import { listBoards } from '../datasource/boardsDatasource'
import { gentasksQueryKeys } from './queryKeys'

export function useBoardList(enabled: boolean) {
  return useQuery({ queryKey: gentasksQueryKeys.boards, queryFn: listBoards, enabled })
}
