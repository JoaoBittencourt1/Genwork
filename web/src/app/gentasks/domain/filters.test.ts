import { describe, expect, it } from 'vitest'
import { boardViewFixture, bugLabel, taskFixture, titlesByColumn } from '@/test/gentasksFixtures'
import {
  EMPTY_FILTER,
  filterBoardView,
  isAssigneeSelected,
  isFilterActive,
  matchesFilter,
  selectedNames,
  toggleAssigneeFilter,
  toggleValue,
  UNASSIGNED,
} from './filters'
import type { BoardView } from './types'

function boardWithVariedTasks(): BoardView {
  const view = boardViewFixture()
  const backlog = view.columns[0]!
  backlog.tasks = [
    taskFixture({ token: 't1', title: 'Urgent bug', urgency: 'urgent', labels: [bugLabel], assigneeToken: 'user-ana' }),
    taskFixture({ token: 't2', title: 'Low chore', urgency: 'low', position: 1 }),
    taskFixture({ token: 't3', title: 'Urgent chore', urgency: 'urgent', position: 2, assigneeToken: 'user-bruno' }),
  ]
  return view
}

describe('filters', () => {
  it('keeps everything with an empty filter', () => {
    const view = boardWithVariedTasks()
    expect(isFilterActive(EMPTY_FILTER)).toBe(false)
    expect(filterBoardView(view, EMPTY_FILTER)).toBe(view)
  })

  it('narrows by urgency keeping the stored order', () => {
    const filtered = filterBoardView(boardWithVariedTasks(), { ...EMPTY_FILTER, urgencies: ['urgent'] })
    expect(titlesByColumn(filtered)['column-backlog']).toEqual(['Urgent bug', 'Urgent chore'])
    expect(titlesByColumn(filtered)['column-todo']).toEqual([])
  })

  it('combines dimensions with AND and values with OR', () => {
    const task = taskFixture({ urgency: 'urgent', labels: [bugLabel] })
    expect(matchesFilter(task, { urgencies: ['urgent', 'low'], labelTokens: ['label-bug'], assigneeTokens: [] })).toBe(true)
    expect(matchesFilter(task, { urgencies: ['urgent'], labelTokens: ['label-feature'], assigneeTokens: [] })).toBe(false)
  })

  it('matches unassigned tasks through the UNASSIGNED value', () => {
    const view = boardWithVariedTasks()
    const filtered = filterBoardView(view, { ...EMPTY_FILTER, assigneeTokens: [UNASSIGNED, 'user-bruno'] })
    expect(titlesByColumn(filtered)['column-backlog']).toEqual(['Low chore', 'Urgent chore'])
  })

  it('toggles values in and out', () => {
    expect(toggleValue(['low'], 'high')).toEqual(['low', 'high'])
    expect(toggleValue(['low', 'high'], 'low')).toEqual(['high'])
  })

  it('names the selected options in order', () => {
    const options = [
      { token: 'a', name: 'Ana' },
      { token: 'b', name: 'Bruno' },
    ]
    expect(selectedNames(['b', 'a', 'gone'], options)).toBe('Bruno, Ana, gone')
  })

  it('toggles and checks an assignee or unassigned filter entry', () => {
    const withAna = toggleAssigneeFilter(EMPTY_FILTER, 'user-ana')
    expect(isAssigneeSelected(withAna, 'user-ana')).toBe(true)
    expect(isAssigneeSelected(withAna, null)).toBe(false)

    const withUnassigned = toggleAssigneeFilter(withAna, null)
    expect(isAssigneeSelected(withUnassigned, null)).toBe(true)
    expect(withUnassigned.assigneeTokens).toEqual(['user-ana', UNASSIGNED])

    const withoutAna = toggleAssigneeFilter(withUnassigned, 'user-ana')
    expect(isAssigneeSelected(withoutAna, 'user-ana')).toBe(false)
    expect(withoutAna.assigneeTokens).toEqual([UNASSIGNED])
  })
})
