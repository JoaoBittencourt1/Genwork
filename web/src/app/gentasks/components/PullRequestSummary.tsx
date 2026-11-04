import { useState } from 'react'
import { pullRequestReference, pullRequestStateKey, pullRequestTitle } from '../domain/pullRequests'
import type { TaskPullRequest } from '../domain/types'
import { PullRequestStateIcon } from './PullRequestStateIcon'

// PullRequestSummary is one pull request as the card menu and the task dialog list it.
export function PullRequestSummary({ pullRequest }: { pullRequest: TaskPullRequest }) {
  return (
    <>
      <PullRequestStateIcon state={pullRequestStateKey(pullRequest)} className="mt-0.5 self-start" />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-sm">{pullRequestTitle(pullRequest)}</span>
        <span className="truncate text-xs text-muted-foreground">{pullRequestReference(pullRequest)}</span>
      </span>
      {pullRequest.author ? <PullRequestAuthorAvatar login={pullRequest.author.login} avatarUrl={pullRequest.author.avatarUrl} /> : null}
    </>
  )
}

// The avatar comes from GitHub's image host; it is left out when the host does not answer.
function PullRequestAuthorAvatar({ login, avatarUrl }: { login: string; avatarUrl: string }) {
  const [failed, setFailed] = useState(false)
  if (failed || !avatarUrl) return null
  return (
    <img
      src={avatarUrl}
      alt={login}
      title={login}
      loading="lazy"
      referrerPolicy="no-referrer"
      className="size-5 shrink-0 self-center rounded-full ring-1 ring-border"
      onError={() => setFailed(true)}
    />
  )
}
