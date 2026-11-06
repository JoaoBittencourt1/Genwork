import { describe, expect, it } from 'vitest'
import type { DirectoryUser } from '@/shared/datasource/users'
import { taskFixture } from '@/test/gentasksFixtures'
import { assigneeGroups, botReviewAvailability, isBotAssignee, reviewerSelection, type BotOption } from './botAssignees'

const ana: DirectoryUser = { token: 'user-ana', kind: 'human', name: 'Ana', email: 'ana@example.com', avatarColor: 'blue' }
const devBot: DirectoryUser = { token: 'member-dev', kind: 'bot', name: 'Dev Bot', email: '', avatarColor: 'purple' }
const reviewBot: DirectoryUser = { token: 'member-review', kind: 'bot', name: 'Review Bot', email: '', avatarColor: 'pink' }
const offDevBot: DirectoryUser = { token: 'member-off', kind: 'bot', name: 'Off Bot', email: '', avatarColor: 'red' }
const users = [ana, devBot, reviewBot, offDevBot]
const options: BotOption[] = [
  { memberToken: 'member-dev', name: 'Dev Bot', kind: 'dev', enabled: true },
  { memberToken: 'member-review', name: 'Review Bot', kind: 'reviewer', enabled: true },
  { memberToken: 'member-off', name: 'Off Bot', kind: 'dev', enabled: false },
]

describe('assigneeGroups', () => {
  it('groups people apart from the enabled dev bots while genagents is enabled', () => {
    const groups = assigneeGroups({ users, botOptions: options, genagentsEnabled: true, currentAssigneeToken: null })

    expect(groups.people).toEqual([ana])
    expect(groups.bots).toEqual([devBot])
  })

  it('offers no bot while genagents is disabled', () => {
    const groups = assigneeGroups({ users, botOptions: options, genagentsEnabled: false, currentAssigneeToken: null })

    expect(groups.bots).toEqual([])
    expect(groups.people).toEqual([ana])
  })

  it('keeps a bot that already holds the task visible either way', () => {
    expect(assigneeGroups({ users, botOptions: options, genagentsEnabled: false, currentAssigneeToken: 'member-review' }).bots).toEqual([reviewBot])
    expect(assigneeGroups({ users, botOptions: undefined, genagentsEnabled: true, currentAssigneeToken: 'member-off' }).bots).toEqual([offDevBot])
  })

  it('recognizes a bot assignee', () => {
    expect(isBotAssignee(taskFixture({ assigneeToken: 'member-dev', assigneeKind: 'bot' }))).toBe(true)
    expect(isBotAssignee(taskFixture({ assigneeToken: 'user-ana', assigneeKind: 'human' }))).toBe(false)
    expect(isBotAssignee(taskFixture())).toBe(false)
  })
})

describe('botReviewAvailability', () => {
  it('is enabled with an open or draft pull request and a reviewer bot', () => {
    expect(botReviewAvailability([{ state: 'open' }], 1)).toEqual({ enabled: true })
    expect(botReviewAvailability([{ state: 'merged' }, { state: 'draft' }], 2)).toEqual({ enabled: true })
  })

  it('says why it is disabled', () => {
    expect(botReviewAvailability([], 1)).toEqual({ enabled: false, reason: 'noOpenPullRequest' })
    expect(botReviewAvailability([{ state: 'merged' }, { state: 'closed' }, { state: null }], 1)).toEqual({ enabled: false, reason: 'noOpenPullRequest' })
    expect(botReviewAvailability([{ state: 'open' }], 0)).toEqual({ enabled: false, reason: 'noReviewerBot' })
  })
})

describe('reviewerSelection', () => {
  it('uses the only reviewer bot without asking and asks when there are several', () => {
    const reviewers = options.filter((bot) => bot.kind === 'reviewer')

    expect(reviewerSelection(reviewers)).toEqual({ needsPicker: false, defaultMemberToken: 'member-review' })
    expect(reviewerSelection([...reviewers, { ...reviewers[0]!, memberToken: 'other' }])).toEqual({ needsPicker: true, defaultMemberToken: null })
    expect(reviewerSelection([])).toEqual({ needsPicker: false, defaultMemberToken: null })
  })
})
