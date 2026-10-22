import type { BoardGithubSettings, BoardGithubSettingsPatch } from './types'

export const MAX_FINAL_BRANCHES = 20
export const MAX_BRANCH_NAME_LENGTH = 255

// GITHUB_COLUMN_FIELDS lists, in screen order, the pull request events a board maps to a column.
export const GITHUB_COLUMN_FIELDS = ['openedColumnToken', 'reviewRequestedColumnToken', 'mergedColumnToken'] as const
export type GithubColumnField = (typeof GITHUB_COLUMN_FIELDS)[number]

export type BoardGithubSettingsDraft = BoardGithubSettings

export type FinalBranchesProblem = 'empty' | 'tooMany' | 'invalidName'

export function draftFromGithubSettings(settings: BoardGithubSettings): BoardGithubSettingsDraft {
  return { ...settings, finalBranches: [...settings.finalBranches] }
}

// buildGithubSettingsPatch sends only what changed, so saving one select never rewrites the others (rule 17).
export function buildGithubSettingsPatch(settings: BoardGithubSettings, draft: BoardGithubSettingsDraft): BoardGithubSettingsPatch {
  const patch: BoardGithubSettingsPatch = {}
  for (const field of GITHUB_COLUMN_FIELDS) {
    if (draft[field] !== settings[field]) patch[field] = draft[field]
  }
  const branchesChanged =
    draft.finalBranches.length !== settings.finalBranches.length || draft.finalBranches.some((branch, index) => branch !== settings.finalBranches[index])
  if (branchesChanged) patch.finalBranches = draft.finalBranches
  return patch
}

// isValidBranchName mirrors the API: 1 to 255 characters and no whitespace; git branch names are case-sensitive.
export function isValidBranchName(branch: string): boolean {
  return branch.length > 0 && [...branch].length <= MAX_BRANCH_NAME_LENGTH && !/\s/.test(branch)
}

export function addFinalBranch(branches: string[], candidate: string): string[] {
  const branch = candidate.trim()
  if (!isValidBranchName(branch) || branches.includes(branch) || branches.length >= MAX_FINAL_BRANCHES) return branches
  return [...branches, branch]
}

export function removeFinalBranch(branches: string[], branch: string): string[] {
  return branches.filter((existing) => existing !== branch)
}

export function finalBranchesProblem(branches: string[]): FinalBranchesProblem | null {
  if (branches.length === 0) return 'empty'
  if (branches.length > MAX_FINAL_BRANCHES) return 'tooMany'
  if (!branches.every(isValidBranchName)) return 'invalidName'
  return null
}
