import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { useErrorMessage } from '@/shared/hooks/useErrorMessage'
import { moveColumn as moveColumnRequest } from '../datasource/boardsDatasource'
import { moveTask as moveTaskRequest } from '../datasource/tasksDatasource'
import { moveColumn, moveTask, type TaskMove } from '../domain/ordering'
import type { BoardView } from '../domain/types'
import { gentasksQueryKeys } from './queryKeys'

type ColumnMove = { columnToken: string; toPosition: number }

// useBoardMoves updates the board as soon as an item is dropped and restores the previous order if the server refuses.
export function useBoardMoves(boardToken: string) {
  const queryClient = useQueryClient()
  const { describeError } = useErrorMessage()
  const queryKey = gentasksQueryKeys.board(boardToken)

  async function applyOptimistically(update: (view: BoardView) => BoardView) {
    await queryClient.cancelQueries({ queryKey })
    const previous = queryClient.getQueryData<BoardView>(queryKey)
    if (previous) queryClient.setQueryData<BoardView>(queryKey, update(previous))
    return { previous }
  }

  const callbacks = {
    onError: (error: unknown, _variables: unknown, context: { previous?: BoardView } | undefined) => {
      if (context?.previous) queryClient.setQueryData(queryKey, context.previous)
      toast.error(describeError(error))
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey }),
  }

  const taskMove = useMutation({
    mutationFn: (move: TaskMove) => moveTaskRequest(move.taskToken, move.toColumnToken, move.toPosition),
    onMutate: (move: TaskMove) => applyOptimistically((view) => moveTask(view, move)),
    ...callbacks,
  })
  const columnMove = useMutation({
    mutationFn: (move: ColumnMove) => moveColumnRequest(move.columnToken, move.toPosition),
    onMutate: (move: ColumnMove) => applyOptimistically((view) => moveColumn(view, move.columnToken, move.toPosition)),
    ...callbacks,
  })

  return { moveTask: taskMove.mutate, moveColumn: columnMove.mutate }
}
