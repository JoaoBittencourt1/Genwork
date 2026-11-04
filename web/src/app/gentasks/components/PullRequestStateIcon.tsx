import { GitMergeIcon, GitPullRequestClosedIcon, GitPullRequestDraftIcon, GitPullRequestIcon, type LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/shared/lib/utils'
import type { PullRequestStateKey } from '../domain/pullRequests'

const STATE_ICONS: Record<PullRequestStateKey, { icon: LucideIcon; className: string }> = {
  open: { icon: GitPullRequestIcon, className: 'text-primary' },
  draft: { icon: GitPullRequestDraftIcon, className: 'text-muted-foreground' },
  merged: { icon: GitMergeIcon, className: 'text-primary' },
  closed: { icon: GitPullRequestClosedIcon, className: 'text-destructive' },
  unknown: { icon: GitPullRequestIcon, className: 'text-muted-foreground' },
}

export function PullRequestStateIcon({ state, className }: { state: PullRequestStateKey; className?: string }) {
  const { t } = useTranslation()
  const { icon: Icon, className: colorClassName } = STATE_ICONS[state]
  return (
    <Icon role="img" aria-label={t(`gentasks.task.pullRequestState.${state}`)} className={cn('size-4 shrink-0', colorClassName, className)} />
  )
}
