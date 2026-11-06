import { useQuery } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { conversationRefreshInterval, lastSequenceOf, mergeEntries, type ActiveRun, type ConversationEntry } from '@/shared/domain/conversation'
import { readChat } from '../datasource/genagentsDatasource'
import { genagentsQueryKeys } from './queryKeys'

function useTabHidden(): boolean {
  const [hidden, setHidden] = useState(document.hidden)
  useEffect(() => {
    const update = () => setHidden(document.hidden)
    document.addEventListener('visibilitychange', update)
    return () => document.removeEventListener('visibilitychange', update)
  }, [])
  return hidden
}

// useChatConversation reads only what is new since the last read and merges it, polling fast while a run is active.
export function useChatConversation(chatToken: string) {
  const [entries, setEntries] = useState<ConversationEntry[]>([])
  const [activeRun, setActiveRun] = useState<ActiveRun | null>(null)
  const [botName, setBotName] = useState('')
  const lastSequence = useRef(0)
  const hidden = useTabHidden()

  const query = useQuery({
    queryKey: genagentsQueryKeys.chat(chatToken),
    queryFn: async () => {
      const page = await readChat(chatToken, lastSequence.current)
      setEntries((shown) => {
        const merged = mergeEntries(shown, page.entries)
        lastSequence.current = lastSequenceOf(merged)
        return merged
      })
      setActiveRun(page.activeRun)
      setBotName(page.botName)
      return page
    },
    refetchInterval: conversationRefreshInterval({ activeRun, hidden }),
  })

  return { entries, activeRun, botName, query, refetch: query.refetch }
}
