import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { ApiError } from '@/shared/datasource/httpClient'
import { checkForUpdates, findUpdateStatus, startUpdate } from '../datasource/updates'
import { deriveUpdateCardView, type UpdateCardContext } from '../domain/updateCard'
import { systemQueryKeys } from './queryKeys'

type CardActions = Pick<UpdateCardContext, 'isChecking' | 'isConfirming' | 'followedUpdate'>

type PollState = { error: unknown; dataUpdatedAt: number; errorUpdatedAt: number }

// pollingContext reads time from the query itself: the last answer, or the last failure, is "now" for the card, and
// the last answer marks when the status became unreadable.
function pollingContext({ error, dataUpdatedAt, errorUpdatedAt }: PollState, actions: CardActions): UpdateCardContext {
  return {
    ...actions,
    lastPollError: error,
    unreachableSince: error ? dataUpdatedAt || errorUpdatedAt : null,
    now: Math.max(dataUpdatedAt, errorUpdatedAt),
  }
}

export function useUpdateStatus(enabled: boolean) {
  const queryClient = useQueryClient()
  const [followedUpdate, setFollowedUpdate] = useState(false)
  const [isConfirming, setConfirming] = useState(false)

  const check = useMutation({
    mutationFn: checkForUpdates,
    onSuccess: (status) => queryClient.setQueryData(systemQueryKeys.update, status),
  })
  const start = useMutation({
    mutationFn: startUpdate,
    onSuccess: (status) => {
      setFollowedUpdate(true)
      queryClient.setQueryData(systemQueryKeys.update, status)
    },
    onError: (error) => {
      if (error instanceof ApiError && error.code === 'version_mismatch') check.mutate()
    },
  })
  const actions: CardActions = { isChecking: check.isPending, isConfirming, followedUpdate }
  const status = useQuery({
    queryKey: systemQueryKeys.update,
    queryFn: async () => {
      const current = await findUpdateStatus()
      if (current.state === 'updating') setFollowedUpdate(true)
      return current
    },
    enabled,
    refetchInterval: (query) => deriveUpdateCardView(query.state.data, pollingContext(query.state, actions)).nextPollDelay ?? false,
  })
  const view = deriveUpdateCardView(status.data, pollingContext(status, actions))

  return {
    view,
    checkError: check.error,
    startError: start.error,
    checkForUpdates: () => check.mutate(),
    askToConfirm: () => setConfirming(true),
    cancelConfirmation: () => setConfirming(false),
    confirmUpdate: (version: string) => {
      setConfirming(false)
      start.mutate(version)
    },
  }
}
