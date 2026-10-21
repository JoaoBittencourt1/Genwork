import { useQuery } from '@tanstack/react-query'
import { listAssignedTasks } from '../datasource/tasksDatasource'
import { gentasksQueryKeys } from './queryKeys'

export function useAssignedTasks(limit: number, enabled: boolean) {
  return useQuery({ queryKey: gentasksQueryKeys.assignedTasks(limit), queryFn: () => listAssignedTasks(limit), enabled })
}
