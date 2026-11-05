import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { MessageSquarePlusIcon, Trash2Icon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/shared/components/ConfirmDialog'
import { ConversationEntries } from '@/shared/components/ConversationEntries'
import { ConversationMessageBox } from '@/shared/components/ConversationMessageBox'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { isRunActive } from '@/shared/domain/conversation'
import { cn } from '@/shared/lib/utils'
import { Button } from '@/shared/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/shared/ui/dialog'
import { Field, FieldLabel } from '@/shared/ui/field'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui/select'
import { Spinner } from '@/shared/ui/spinner'
import { deleteChat, listBots, listChats, reopenChat, sendChatMessage, startChat, stopChat } from '../datasource/genagentsDatasource'
import { botsOfKind } from '../domain/providers'
import type { ChatSummary } from '../domain/types'
import { genagentsQueryKeys } from './queryKeys'
import { useChatConversation } from './useChatConversation'

export function AdminChatPanel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const chats = useQuery({ queryKey: genagentsQueryKeys.chats, queryFn: listChats })
  const [selected, setSelected] = useState<string | null>(null)
  const [starting, setStarting] = useState(false)
  const [deleting, setDeleting] = useState<ChatSummary | null>(null)
  const refreshChats = () => queryClient.invalidateQueries({ queryKey: genagentsQueryKeys.chats })
  const remove = useMutation({
    mutationFn: (chat: ChatSummary) => deleteChat(chat.token),
    onSuccess: (_, chat) => {
      if (selected === chat.token) setSelected(null)
      void refreshChats()
    },
  })

  if (chats.isPending) return <Spinner />
  if (chats.isLoadingError) return <ErrorAlert error={chats.error} />

  return (
    <div className="grid gap-4 md:grid-cols-[16rem_1fr]">
      <aside className="flex flex-col gap-2">
        <Button onClick={() => setStarting(true)}>
          <MessageSquarePlusIcon />
          {t('genagents.adminChat.newChat')}
        </Button>
        <ErrorAlert error={remove.error} />
        <ul className="flex flex-col gap-1">
          {chats.data.map((chat) => (
            <li key={chat.token} className="flex items-center gap-1">
              <button
                type="button"
                className={cn('min-w-0 flex-1 truncate rounded-md px-2 py-1.5 text-left text-sm hover:bg-muted', selected === chat.token && 'bg-muted font-medium')}
                onClick={() => setSelected(chat.token)}
              >
                {chat.title}
              </button>
              <Button size="sm" variant="ghost" aria-label={t('genagents.adminChat.delete', { title: chat.title })} onClick={() => setDeleting(chat)}>
                <Trash2Icon />
              </Button>
            </li>
          ))}
        </ul>
      </aside>
      <section>
        {selected ? <ChatConversation key={selected} chatToken={selected} /> : <p className="text-sm text-muted-foreground">{t('genagents.adminChat.pickAChat')}</p>}
      </section>
      <Dialog open={starting} onOpenChange={setStarting}>
        <DialogContent>{starting ? <StartChatForm onDone={() => setStarting(false)} onStarted={(chat) => { setSelected(chat.token); void refreshChats() }} /> : null}</DialogContent>
      </Dialog>
      <ConfirmDialog
        open={deleting !== null}
        title={t('genagents.adminChat.deleteTitle')}
        description={t('genagents.adminChat.deleteDescription', { title: deleting?.title ?? '' })}
        confirmLabel={t('common.delete')}
        onOpenChange={(open) => !open && setDeleting(null)}
        onConfirm={() => {
          if (deleting) remove.mutate(deleting)
          setDeleting(null)
        }}
      />
    </div>
  )
}

function StartChatForm({ onDone, onStarted }: { onDone: () => void; onStarted: (chat: ChatSummary) => void }) {
  const { t } = useTranslation()
  const bots = useQuery({ queryKey: genagentsQueryKeys.bots, queryFn: () => listBots() })
  const assistants = botsOfKind(bots.data ?? [], 'assistant')
  const [botToken, setBotToken] = useState('')
  const start = useMutation({
    mutationFn: () => startChat(botToken),
    onSuccess: (chat) => {
      onStarted(chat)
      onDone()
    },
  })

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(event) => {
        event.preventDefault()
        start.mutate()
      }}
    >
      <DialogHeader>
        <DialogTitle>{t('genagents.adminChat.newChat')}</DialogTitle>
      </DialogHeader>
      <ErrorAlert error={start.error} />
      <Field>
        <FieldLabel htmlFor="chat-bot">{t('genagents.adminChat.assistant')}</FieldLabel>
        <Select value={botToken} onValueChange={setBotToken}>
          <SelectTrigger id="chat-bot">
            <SelectValue placeholder={t('genagents.adminChat.chooseAssistant')} />
          </SelectTrigger>
          <SelectContent>
            {assistants.map((bot) => (
              <SelectItem key={bot.token} value={bot.token}>
                {bot.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onDone}>
          {t('common.cancel')}
        </Button>
        <Button type="submit" disabled={start.isPending || botToken === ''}>
          {t('genagents.adminChat.start')}
        </Button>
      </DialogFooter>
    </form>
  )
}

function ChatConversation({ chatToken }: { chatToken: string }) {
  const { t } = useTranslation()
  const { entries, activeRun, botName, query, refetch } = useChatConversation(chatToken)
  const send = useMutation({ mutationFn: (text: string) => sendChatMessage(chatToken, text), onSuccess: () => void refetch() })
  const stop = useMutation({ mutationFn: () => stopChat(chatToken), onSuccess: () => void refetch() })
  const reopen = useMutation({ mutationFn: () => reopenChat(chatToken), onSuccess: () => void refetch() })
  const sessionEnded = entries.at(-1)?.payload.reason === 'session_revoked'

  if (query.isPending) return <Spinner />
  if (query.isLoadingError) return <ErrorAlert error={query.error} />

  return (
    <div className="flex flex-col gap-3">
      <p className="text-xs text-muted-foreground">{t('genagents.adminChat.with', { name: botName })}</p>
      <ErrorAlert error={send.error ?? stop.error ?? reopen.error} />
      <ConversationEntries entries={entries} authorName={() => t('genagents.adminChat.you')} />
      {sessionEnded ? (
        <div className="flex items-center gap-2 rounded-md border px-3 py-2 text-sm">
          <span className="flex-1">{t('genagents.adminChat.sessionEnded')}</span>
          <Button size="sm" onClick={() => reopen.mutate()}>
            {t('genagents.adminChat.reopen')}
          </Button>
        </div>
      ) : null}
      <ConversationMessageBox disabled={false} isSending={send.isPending} onStop={activeRun && isRunActive(activeRun.state) ? () => stop.mutate() : undefined} onSend={(text) => send.mutate(text)} />
    </div>
  )
}
