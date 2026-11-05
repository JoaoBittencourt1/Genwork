import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ConversationEntries } from '@/shared/components/ConversationEntries'
import { ConversationMessageBox } from '@/shared/components/ConversationMessageBox'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { isRunActive } from '@/shared/domain/conversation'
import type { DirectoryUser } from '@/shared/datasource/users'
import { postTaskMessage, stopTaskRun } from '../datasource/botConversationDatasource'
import { useBotAssignment } from './BotAssignmentContext'
import { useTaskConversation } from './useTaskConversation'

type BotConversationPanelProps = {
  taskToken: string
  users: DirectoryUser[] | undefined
}

// BotConversationPanel shows what the bot working on the task reports, and lets people talk to it. It shows nothing while
// genagents is disabled, and nothing for a task no bot ever worked on.
export function BotConversationPanel({ taskToken, users }: BotConversationPanelProps) {
  const { genagentsEnabled, canReadRuns, canWriteRuns } = useBotAssignment()
  if (!genagentsEnabled || !canReadRuns) return null
  return <ConversationPanel taskToken={taskToken} users={users} canWrite={canWriteRuns} />
}

function ConversationPanel({ taskToken, users, canWrite }: BotConversationPanelProps & { canWrite: boolean }) {
  const { t } = useTranslation()
  const { entries, activeRun, query } = useTaskConversation(taskToken, true)
  const send = useMutation({ mutationFn: (text: string) => postTaskMessage(taskToken, text), onSuccess: () => void query.refetch() })
  const stop = useMutation({ mutationFn: () => stopTaskRun(taskToken), onSuccess: () => void query.refetch() })
  const authorName = (token: string | null) => users?.find((user) => user.token === token)?.name ?? t('gentasks.bot.someone')

  if (query.isPending || (entries.length === 0 && !activeRun)) return null

  return (
    <section className="mt-4 flex flex-col gap-2 border-t pt-4" aria-label={t('gentasks.bot.conversation')}>
      <h3 className="text-sm font-medium">{t('gentasks.bot.conversation')}</h3>
      {activeRun ? <p className="text-xs text-muted-foreground">{t(`gentasks.bot.run.${activeRun.state}`, { bot: activeRun.botName })}</p> : null}
      <ErrorAlert error={send.error ?? stop.error} />
      <ConversationEntries entries={entries} authorName={authorName} />
      {canWrite ? (
        <ConversationMessageBox
          disabled={false}
          isSending={send.isPending}
          onStop={activeRun && isRunActive(activeRun.state) ? () => stop.mutate() : undefined}
          onSend={(text) => send.mutate(text)}
        />
      ) : null}
    </section>
  )
}
