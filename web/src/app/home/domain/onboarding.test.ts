import { describe, expect, it } from 'vitest'
import { PERMISSIONS } from '@/app/auth/domain/session'
import { GENTASKS_PERMISSIONS } from '@/app/gentasks/domain/permissions'
import { buildOnboardingChecklist, type OnboardingSignals } from './onboarding'

const everyPermission = [...Object.values(PERMISSIONS), ...Object.values(GENTASKS_PERMISSIONS)]

function signals(overrides: Partial<OnboardingSignals> = {}): OnboardingSignals {
  return {
    gentasksEnabled: true,
    can: (permission) => everyPermission.includes(permission as (typeof everyPermission)[number]),
    boardCount: 0,
    openAssignedTaskCount: 0,
    directoryUserCount: 1,
    ...overrides,
  }
}

describe('buildOnboardingChecklist', () => {
  it('shows every step as open on a fresh installation', () => {
    const checklist = buildOnboardingChecklist(signals())

    expect(checklist.steps).toEqual([
      { id: 'createBoard', done: false, linkTo: '/tasks' },
      { id: 'getAssignedTask', done: false, linkTo: '/tasks' },
      { id: 'inviteTeam', done: false, linkTo: '/members' },
    ])
    expect(checklist).toMatchObject({ doneCount: 0, totalCount: 3, isComplete: false })
  })

  it('checks steps off from real data', () => {
    const checklist = buildOnboardingChecklist(signals({ boardCount: 2, openAssignedTaskCount: 1, directoryUserCount: 3 }))

    expect(checklist.steps.every((step) => step.done)).toBe(true)
    expect(checklist).toMatchObject({ doneCount: 3, totalCount: 3, isComplete: true })
  })

  it('treats data that did not load as not done', () => {
    const checklist = buildOnboardingChecklist(
      signals({ boardCount: undefined, openAssignedTaskCount: undefined, directoryUserCount: undefined }),
    )

    expect(checklist.doneCount).toBe(0)
  })

  it('hides gentasks steps while gentasks is disabled', () => {
    const checklist = buildOnboardingChecklist(signals({ gentasksEnabled: false, boardCount: 4 }))

    expect(checklist.steps.map((step) => step.id)).toEqual(['inviteTeam'])
  })

  it('hides steps the user is not allowed to do', () => {
    const member = (permission: string) => permission === GENTASKS_PERMISSIONS.boardRead || permission === GENTASKS_PERMISSIONS.taskWrite
    const checklist = buildOnboardingChecklist(signals({ can: member }))

    expect(checklist.steps.map((step) => step.id)).toEqual(['getAssignedTask'])
    expect(checklist.totalCount).toBe(1)
  })

  it('has no steps and is not complete when nothing is visible', () => {
    const checklist = buildOnboardingChecklist(signals({ gentasksEnabled: false, can: () => false }))

    expect(checklist).toEqual({ steps: [], doneCount: 0, totalCount: 0, isComplete: false })
  })
})
