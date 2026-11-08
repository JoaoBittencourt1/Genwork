import { useQuery } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { conversationRefreshInterval, lastSequenceOf, mergeEntries, type ActiveRun, type ConversationEntry } from '@/shared/domain/conversation'
import { readTaskConversation } from '../datasource/botConversationDatasource'

function useTabHidden(): boolean {
  const [hidden, setHidden] = useState(document.hidden)
  useEffect(() => {
    const update = () => setHidden(document.hidden)
    document.addEventListener('visibilitychange', update)
    return () => document.removeEventListener('visibilitychange', update)
  }, [])
  return hidden
}

// useTaskConversation reads only what is new since the last read, polling every 2 seconds while a run is active and every
// 10 seconds otherwise.
export function useTaskConversation(taskToken: string, enabled: boolean) {
  const [entries, setEntries] = useState<ConversationEntry[]>([])
  const [activeRun, setActiveRun] = useState<ActiveRun | null>(null)
  const lastSequence = useRef(0)
  const hidden = useTabHidden()

  const query = useQuery({
    queryKey: ['gentasks', 'task-conversation', taskToken],
    queryFn: async () => {
      const page = await readTaskConversation(taskToken, lastSequence.current)
      setEntries((shown) => {
        const merged = mergeEntries(shown, page.entries)
        lastSequence.current = lastSequenceOf(merged)
        return merged
      })
      setActiveRun(page.activeRun)
      return page
    },
    enabled,
    refetchInterval: conversationRefreshInterval({ activeRun, hidden }),
  })

  // reset forgets what was shown, for a conversation that was just cleared, and reads it again from the start.
  const reset = () => {
    lastSequence.current = 0
    setEntries([])
    setActiveRun(null)
    void query.refetch()
  }

  return { entries, activeRun, query, reset }
}
