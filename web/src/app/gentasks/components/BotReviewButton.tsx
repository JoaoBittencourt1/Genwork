import { useMutation, useQueryClient } from '@tanstack/react-query'
import { BotIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { Button } from '@/shared/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui/select'
import { requestBotReview } from '../datasource/botConversationDatasource'
import { botReviewAvailability, reviewerSelection } from '../domain/botAssignees'
import type { TaskPullRequest } from '../domain/types'
import { useBotAssignment } from './BotAssignmentContext'

type BotReviewButtonProps = {
  taskToken: string
  pullRequests: TaskPullRequest[]
}

// BotReviewButton asks a reviewer bot to review the task's linked pull requests. It is disabled, with the reason, when the
// task has no open or draft pull request, and shows a picker only when there is more than one reviewer bot.
export function BotReviewButton({ taskToken, pullRequests }: BotReviewButtonProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { genagentsEnabled, canWriteRuns, reviewerBots } = useBotAssignment()
  const [chosen, setChosen] = useState('')
  const selection = reviewerSelection(reviewerBots.map((bot) => ({ memberToken: bot.memberToken, name: bot.name, kind: 'reviewer' as const, enabled: true })))
  const request = useMutation({
    mutationFn: () => requestBotReview(taskToken, selection.needsPicker ? reviewerBots.find((bot) => bot.memberToken === chosen)?.token : undefined),
    onSuccess: () => {
      toast.success(t('gentasks.bot.review.requested'))
      void queryClient.invalidateQueries({ queryKey: ['gentasks', 'task-conversation', taskToken] })
    },
  })
  if (!genagentsEnabled || !canWriteRuns) return null
  const availability = botReviewAvailability(pullRequests, reviewerBots.length)
  const needsChoice = selection.needsPicker && chosen === ''

  return (
    <div className="flex flex-col gap-2 px-2 pt-3">
      {selection.needsPicker ? (
        <Select value={chosen} onValueChange={setChosen}>
          <SelectTrigger size="sm" aria-label={t('gentasks.bot.review.reviewer')}>
            <SelectValue placeholder={t('gentasks.bot.review.chooseReviewer')} />
          </SelectTrigger>
          <SelectContent>
            {reviewerBots.map((bot) => (
              <SelectItem key={bot.memberToken} value={bot.memberToken}>
                {bot.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : null}
      <Button type="button" variant="outline" size="sm" disabled={!availability.enabled || needsChoice || request.isPending} onClick={() => request.mutate()}>
        <BotIcon />
        {t('gentasks.bot.review.button')}
      </Button>
      {availability.enabled ? null : <p className="text-xs text-muted-foreground">{t(`gentasks.bot.review.disabled.${availability.reason}`)}</p>}
      <ErrorAlert error={request.error} />
    </div>
  )
}
