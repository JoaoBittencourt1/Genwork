import type { PullRequestState, TaskPullRequest } from './types'

// unknown is the state of a pull request linked before gentasks kept its details.
export type PullRequestStateKey = PullRequestState | 'unknown'

const STATE_ORDER: Record<PullRequestStateKey, number> = { open: 0, draft: 1, unknown: 2, merged: 3, closed: 4 }

export function pullRequestStateKey(pullRequest: TaskPullRequest): PullRequestStateKey {
  return pullRequest.state ?? 'unknown'
}

// sortPullRequests puts the pull requests still in progress first, newest first within a state.
export function sortPullRequests(pullRequests: readonly TaskPullRequest[]): TaskPullRequest[] {
  return [...pullRequests].sort(
    (left, right) =>
      STATE_ORDER[pullRequestStateKey(left)] - STATE_ORDER[pullRequestStateKey(right)] || right.number - left.number,
  )
}

export function pullRequestReference(pullRequest: TaskPullRequest): string {
  return `${pullRequest.repository}#${pullRequest.number}`
}

export function pullRequestTitle(pullRequest: TaskPullRequest): string {
  return pullRequest.title ?? pullRequestReference(pullRequest)
}
