import type { DirectoryUser } from '@/shared/datasource/users'
import type { Task } from './types'

// BotOption is what the assignee picker needs of a genagents bot: the member it assigns tasks to.
export type BotOption = { memberToken: string; name: string; kind: 'dev' | 'reviewer' | 'assistant'; enabled: boolean }

export type AssigneeGroups = { people: DirectoryUser[]; bots: DirectoryUser[] }

// assigneeGroups groups the directory for the picker. While genagents is enabled only the members of enabled dev bots are
// offered; while it is disabled no bot is offered. A bot that already holds the task stays visible either way.
export function assigneeGroups({
  users,
  botOptions,
  genagentsEnabled,
  currentAssigneeToken,
}: {
  users: DirectoryUser[]
  botOptions: BotOption[] | undefined
  genagentsEnabled: boolean
  currentAssigneeToken: string | null
}): AssigneeGroups {
  const people = users.filter((user) => user.kind === 'human')
  const offered = new Set(genagentsEnabled ? (botOptions ?? []).filter((bot) => bot.kind === 'dev' && bot.enabled).map((bot) => bot.memberToken) : [])
  const bots = users.filter((user) => user.kind === 'bot' && (offered.has(user.token) || user.token === currentAssigneeToken))
  return { people, bots }
}

export function isBotAssignee(task: Pick<Task, 'assigneeKind'>): boolean {
  return task.assigneeKind === 'bot'
}

export type BotReviewAvailability = { enabled: true } | { enabled: false; reason: 'noOpenPullRequest' | 'noReviewerBot' }

type PullRequestLike = { state: 'open' | 'draft' | 'merged' | 'closed' | null }

// botReviewAvailability enables "Bot review" only with an open or draft pull request and a reviewer bot to ask.
export function botReviewAvailability(pullRequests: PullRequestLike[], reviewerCount: number): BotReviewAvailability {
  if (!pullRequests.some((pullRequest) => pullRequest.state === 'open' || pullRequest.state === 'draft')) {
    return { enabled: false, reason: 'noOpenPullRequest' }
  }
  if (reviewerCount === 0) return { enabled: false, reason: 'noReviewerBot' }
  return { enabled: true }
}

// reviewerSelection decides whether the dialog needs a picker: with one reviewer bot it is used without asking.
export function reviewerSelection(reviewers: BotOption[]): { needsPicker: boolean; defaultMemberToken: string | null } {
  return { needsPicker: reviewers.length > 1, defaultMemberToken: reviewers.length === 1 ? (reviewers[0]?.memberToken ?? null) : null }
}
