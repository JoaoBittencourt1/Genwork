import { describe, expect, it } from 'vitest'
import { usersFixture } from '@/test/gentasksFixtures'
import { findAssignee, initials } from './assignees'
import { canClearColumnName, columnStatus, columnTitle, resolveColumnTitle, statusColorVariable } from './columnName'
import { labelColorTranslationKey, labelColorVariable, urgencyColorVariable } from './labelPalette'
import { gentasksAbilities } from './permissions'
import { isUrgencyShown, urgencyRank, urgencyTranslationKey } from './urgency'

describe('column titles', () => {
  it('shows the localized preset label until a custom name is set', () => {
    expect(columnTitle({ preset: 'todo', name: null })).toEqual({ kind: 'translated', key: 'gentasks.columnPresets.todo' })
    expect(columnTitle({ preset: 'todo', name: 'Next up' })).toEqual({ kind: 'custom', name: 'Next up' })
    expect(resolveColumnTitle({ preset: 'done', name: null }, (key) => `t:${key}`)).toBe('t:gentasks.columnPresets.done')
    expect(resolveColumnTitle({ preset: null, name: 'Review' }, (key) => key)).toBe('Review')
  })

  it('shows a status for each preset and treats custom columns as in progress', () => {
    expect(columnStatus({ preset: 'backlog' })).toBe('backlog')
    expect(columnStatus({ preset: 'todo' })).toBe('todo')
    expect(columnStatus({ preset: 'done' })).toBe('done')
    expect(columnStatus({ preset: null })).toBe('started')
    expect(statusColorVariable('done')).toBe('var(--status-done)')
  })

  it('lets only renamed preset columns clear their name', () => {
    expect(canClearColumnName({ preset: 'todo', name: 'Next up' })).toBe(true)
    expect(canClearColumnName({ preset: 'todo', name: null })).toBe(false)
    expect(canClearColumnName({ preset: null, name: 'Review' })).toBe(false)
  })
})

describe('urgency and palette', () => {
  it('orders urgency levels and maps them to keys and theme variables', () => {
    expect(urgencyRank('none')).toBeLessThan(urgencyRank('urgent'))
    expect(urgencyTranslationKey('high')).toBe('gentasks.urgency.high')
    expect(isUrgencyShown('none')).toBe(false)
    expect(isUrgencyShown('low')).toBe(true)
    expect(urgencyColorVariable('urgent')).toBe('var(--urgency-urgent)')
  })

  it('maps label colors to theme variables and translation keys', () => {
    expect(labelColorVariable('teal')).toBe('var(--label-teal)')
    expect(labelColorTranslationKey('pink')).toBe('gentasks.labelColors.pink')
  })
})

describe('abilities and assignees', () => {
  it('derives abilities from permissions', () => {
    const granted = new Set(['gentasks.board.read', 'gentasks.task.write', 'gentasks.github.manage'])
    expect(gentasksAbilities((permission) => granted.has(permission))).toEqual({
      canRead: true,
      canManageBoards: false,
      canWriteTasks: true,
      canDeleteTasks: false,
      canManageGithub: true,
    })
  })

  it('finds assignees and builds initials', () => {
    expect(findAssignee(usersFixture, 'user-bruno')?.name).toBe('Bruno')
    expect(findAssignee(usersFixture, null)).toBeUndefined()
    expect(findAssignee(undefined, 'user-ana')).toBeUndefined()
    expect(initials('Ana Souza')).toBe('AS')
    expect(initials('Bruno')).toBe('BR')
  })
})
