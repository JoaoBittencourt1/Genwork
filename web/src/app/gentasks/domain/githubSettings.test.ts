import { describe, expect, it } from 'vitest'
import { boardGithubSettingsFixture } from '@/test/gentasksFixtures'
import {
  addFinalBranch,
  buildGithubSettingsPatch,
  draftFromGithubSettings,
  finalBranchesProblem,
  isValidBranchName,
  removeFinalBranch,
} from './githubSettings'

describe('github settings', () => {
  const settings = boardGithubSettingsFixture()

  it('sends nothing when nothing changed', () => {
    expect(buildGithubSettingsPatch(settings, draftFromGithubSettings(settings))).toEqual({})
  })

  it('sends only the changed fields, with null for "do not move"', () => {
    const draft = { ...draftFromGithubSettings(settings), reviewRequestedColumnToken: 'column-review', mergedColumnToken: null }
    expect(buildGithubSettingsPatch(settings, draft)).toEqual({ reviewRequestedColumnToken: 'column-review', mergedColumnToken: null })
  })

  it('sends the whole branch list when it changed', () => {
    const draft = { ...draftFromGithubSettings(settings), finalBranches: ['main', 'master', 'staging'] }
    expect(buildGithubSettingsPatch(settings, draft)).toEqual({ finalBranches: ['main', 'master', 'staging'] })
    expect(buildGithubSettingsPatch(settings, { ...draft, finalBranches: ['master', 'main'] })).toEqual({ finalBranches: ['master', 'main'] })
  })

  it('validates branch names', () => {
    expect(isValidBranchName('release/2026-10')).toBe(true)
    expect(isValidBranchName('')).toBe(false)
    expect(isValidBranchName('feature x')).toBe(false)
    expect(isValidBranchName('ma\tin')).toBe(false)
    expect(isValidBranchName('b'.repeat(256))).toBe(false)
  })

  it('adds trimmed, new, valid branches only', () => {
    expect(addFinalBranch(['main'], ' staging ')).toEqual(['main', 'staging'])
    expect(addFinalBranch(['main'], 'main')).toEqual(['main'])
    expect(addFinalBranch(['main'], 'Main')).toEqual(['main', 'Main'])
    expect(addFinalBranch(['main'], 'not valid')).toEqual(['main'])
    expect(addFinalBranch(Array.from({ length: 20 }, (_, index) => `release-${index}`), 'one-more')).toHaveLength(20)
  })

  it('removes branches', () => {
    expect(removeFinalBranch(['main', 'master'], 'main')).toEqual(['master'])
  })

  it('reports why a branch list cannot be saved', () => {
    expect(finalBranchesProblem(['main'])).toBeNull()
    expect(finalBranchesProblem([])).toBe('empty')
    expect(finalBranchesProblem(Array.from({ length: 21 }, (_, index) => `release-${index}`))).toBe('tooMany')
    expect(finalBranchesProblem(['main', 'feature x'])).toBe('invalidName')
  })
})
