import { PERMISSIONS } from '@/app/auth/domain/session'
import { GENTASKS_PERMISSIONS } from '@/app/gentasks/domain/permissions'

export const ONBOARDING_STEP_IDS = ['createBoard', 'getAssignedTask', 'inviteTeam'] as const
export type OnboardingStepId = (typeof ONBOARDING_STEP_IDS)[number]

export type OnboardingStepLink = '/tasks' | '/members'

export type OnboardingStep = {
  id: OnboardingStepId
  done: boolean
  linkTo: OnboardingStepLink
}

// A count is undefined while its data is loading, failed, or is not allowed; the step then reads as not done.
export type OnboardingSignals = {
  gentasksEnabled: boolean
  can: (permission: string) => boolean
  boardCount: number | undefined
  openAssignedTaskCount: number | undefined
  directoryUserCount: number | undefined
}

export type OnboardingChecklist = {
  steps: OnboardingStep[]
  doneCount: number
  totalCount: number
  isComplete: boolean
}

type StepRule = {
  id: OnboardingStepId
  linkTo: OnboardingStepLink
  isVisible: (signals: OnboardingSignals) => boolean
  isDone: (signals: OnboardingSignals) => boolean
}

const STEP_RULES: StepRule[] = [
  {
    id: 'createBoard',
    linkTo: '/tasks',
    isVisible: ({ gentasksEnabled, can }) => gentasksEnabled && can(GENTASKS_PERMISSIONS.boardManage),
    isDone: ({ boardCount }) => (boardCount ?? 0) > 0,
  },
  {
    id: 'getAssignedTask',
    linkTo: '/tasks',
    isVisible: ({ gentasksEnabled, can }) =>
      gentasksEnabled && can(GENTASKS_PERMISSIONS.boardRead) && can(GENTASKS_PERMISSIONS.taskWrite),
    isDone: ({ openAssignedTaskCount }) => (openAssignedTaskCount ?? 0) > 0,
  },
  {
    id: 'inviteTeam',
    linkTo: '/members',
    isVisible: ({ can }) => can(PERMISSIONS.inviteCreate) && can(PERMISSIONS.userRead),
    isDone: ({ directoryUserCount }) => (directoryUserCount ?? 0) > 1,
  },
]

export function buildOnboardingChecklist(signals: OnboardingSignals): OnboardingChecklist {
  const steps = STEP_RULES.filter((rule) => rule.isVisible(signals)).map((rule) => ({
    id: rule.id,
    done: rule.isDone(signals),
    linkTo: rule.linkTo,
  }))
  const doneCount = steps.filter((step) => step.done).length
  return { steps, doneCount, totalCount: steps.length, isComplete: steps.length > 0 && doneCount === steps.length }
}
