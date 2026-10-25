import { GitPullRequestIcon } from 'lucide-react'
import { memo } from 'react'
import { useTranslation } from 'react-i18next'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/shared/ui/dropdown-menu'
import { sortPullRequests } from '../domain/pullRequests'
import type { TaskPullRequest } from '../domain/types'
import { PullRequestSummary } from './PullRequestSummary'

type TaskPullRequestsMenuProps = {
  taskIdentifier: string
  pullRequests: TaskPullRequest[]
}

// Like TaskAssigneeMenu, the chip sits beside the card's own button and keeps pointer, click, and key events away from
// the card: they would open the task or start a drag.
export const TaskPullRequestsMenu = memo(function TaskPullRequestsMenu({ taskIdentifier, pullRequests }: TaskPullRequestsMenuProps) {
  const { t } = useTranslation()
  const keepFromCard = (event: { stopPropagation: () => void }) => event.stopPropagation()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={t('gentasks.task.openPullRequests', { task: taskIdentifier })}
        className="absolute right-2 bottom-2 inline-flex h-6 items-center gap-1 rounded-md border bg-board-card px-1.5 text-xs text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
        onPointerDown={keepFromCard}
        onClick={keepFromCard}
        onKeyDown={keepFromCard}
      >
        <GitPullRequestIcon aria-hidden className="size-3.5 text-primary" />
        {t('gentasks.task.pullRequestCount', { count: pullRequests.length })}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-80 max-w-[calc(100vw-2rem)]" onPointerDown={keepFromCard} onClick={keepFromCard} onKeyDown={keepFromCard}>
        {sortPullRequests(pullRequests).map((pullRequest) => (
          <DropdownMenuItem key={pullRequest.token} asChild className="items-start gap-2">
            <a href={pullRequest.url} target="_blank" rel="noopener noreferrer">
              <PullRequestSummary pullRequest={pullRequest} />
            </a>
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
})
