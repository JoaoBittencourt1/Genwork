import { BotIcon, WrenchIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { entryMessageKey, entryParams, entryReviewLinks, entryText, type ConversationEntry } from '@/shared/domain/conversation'
import { cn } from '@/shared/lib/utils'

type ConversationEntriesProps = {
  entries: ConversationEntry[]
  // authorName resolves a member token to a name, for people's messages.
  authorName: (token: string | null) => string
}

// ConversationEntries lists what happened in a conversation, in order. System entries carry message keys, localized here.
export function ConversationEntries({ entries, authorName }: ConversationEntriesProps) {
  const { t } = useTranslation()
  return (
    <ol className="flex flex-col gap-2" aria-label={t('conversation.entries')}>
      {entries.map((entry) => (
        <li key={entry.sequence} className={cn('flex flex-col gap-1 rounded-md border px-3 py-2 text-sm', entry.type === 'human_message' && 'bg-muted/50')}>
          <ConversationEntryBody entry={entry} authorName={authorName} />
        </li>
      ))}
    </ol>
  )
}

function ConversationEntryBody({ entry, authorName }: { entry: ConversationEntry; authorName: (token: string | null) => string }) {
  const { t } = useTranslation()
  switch (entry.type) {
    case 'human_message':
      return (
        <>
          <span className="text-xs font-medium text-muted-foreground">{authorName(entry.authorToken)}</span>
          <p className="whitespace-pre-wrap">{entryText(entry)}</p>
        </>
      )
    case 'assistant_message':
    case 'result':
      return (
        <>
          <span className="flex items-center gap-1 text-xs font-medium text-muted-foreground">
            <BotIcon className="size-3" aria-hidden />
            {t(entry.type === 'result' ? 'conversation.result' : 'conversation.assistant')}
          </span>
          <p className="whitespace-pre-wrap">{entryText(entry) || t('conversation.noText')}</p>
          {entryReviewLinks(entry).map((url) => (
            <a key={url} className="text-xs underline" href={url} target="_blank" rel="noreferrer">
              {url}
            </a>
          ))}
          {typeof entry.payload.pullRequestUrl === 'string' ? (
            <a className="text-xs underline" href={entry.payload.pullRequestUrl} target="_blank" rel="noreferrer">
              {entry.payload.pullRequestUrl}
            </a>
          ) : null}
        </>
      )
    case 'tool_use':
      return (
        <span className="flex items-center gap-1 text-xs text-muted-foreground">
          <WrenchIcon className="size-3" aria-hidden />
          {typeof entry.payload.summary === 'string' ? entry.payload.summary : String(entry.payload.tool ?? '')}
        </span>
      )
    default: {
      const params = entryParams(entry)
      const reasonText = typeof params.reason === 'string' ? t(`conversation.reason.${params.reason}`, { defaultValue: params.reason, ...params }) : ''
      const causeText = typeof params.cause === 'string' ? t(`conversation.cause.${params.cause}`, { defaultValue: params.cause }) : ''
      return (
        <span className={cn('text-xs', entry.type === 'error' ? 'text-destructive' : 'text-muted-foreground')}>
          {t(entryMessageKey(entry), { defaultValue: t('conversation.entry.unknown'), ...params, reasonText, causeText })}
        </span>
      )
    }
  }
}
