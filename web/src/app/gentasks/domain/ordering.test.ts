import { describe, expect, it } from 'vitest'
import { boardViewFixture, taskFixture, titlesByColumn } from '@/test/gentasksFixtures'
import { findTaskLocation, haveSameContent, isSameLocation, moveColumn, moveTask } from './ordering'

describe('moveTask', () => {
  it('moves a task to another column and renumbers both columns', () => {
    const view = boardViewFixture()

    const moved = moveTask(view, { taskToken: 'task-A', toColumnToken: 'column-todo', toPosition: 1 })

    expect(titlesByColumn(moved)).toEqual({ 'column-backlog': ['B', 'C'], 'column-todo': ['D', 'A'], 'column-done': [] })
    expect(moved.columns[0]?.tasks.map((task) => task.position)).toEqual([0, 1])
    expect(moved.columns[1]?.tasks[1]).toMatchObject({ columnToken: 'column-todo', position: 1 })
  })

  it('reorders within a column', () => {
    const moved = moveTask(boardViewFixture(), { taskToken: 'task-C', toColumnToken: 'column-backlog', toPosition: 0 })
    expect(titlesByColumn(moved)['column-backlog']).toEqual(['C', 'A', 'B'])
  })

  it('clamps positions past the end and below zero', () => {
    const view = boardViewFixture()
    expect(titlesByColumn(moveTask(view, { taskToken: 'task-A', toColumnToken: 'column-done', toPosition: 9 }))['column-done']).toEqual(['A'])
    expect(titlesByColumn(moveTask(view, { taskToken: 'task-D', toColumnToken: 'column-backlog', toPosition: -3 }))['column-backlog']).toEqual(['D', 'A', 'B', 'C'])
  })

  it('leaves the view and the original untouched for unknown tokens', () => {
    const view = boardViewFixture()
    expect(moveTask(view, { taskToken: 'missing', toColumnToken: 'column-todo', toPosition: 0 })).toBe(view)
    expect(moveTask(view, { taskToken: 'task-A', toColumnToken: 'missing', toPosition: 0 })).toBe(view)
    moveTask(view, { taskToken: 'task-A', toColumnToken: 'column-todo', toPosition: 0 })
    expect(titlesByColumn(view)['column-backlog']).toEqual(['A', 'B', 'C'])
  })

  it('finds where a task is and recognises no-op moves', () => {
    const view = boardViewFixture()
    expect(findTaskLocation(view, 'task-D')).toEqual({ columnToken: 'column-todo', index: 0 })
    expect(findTaskLocation(view, 'missing')).toBeUndefined()
    expect(isSameLocation(view, { taskToken: 'task-B', toColumnToken: 'column-backlog', toPosition: 1 })).toBe(true)
    expect(isSameLocation(view, { taskToken: 'task-B', toColumnToken: 'column-todo', toPosition: 1 })).toBe(false)
  })
})

describe('moveColumn', () => {
  it('moves a column and renumbers every column', () => {
    const moved = moveColumn(boardViewFixture(), 'column-done', 0)
    expect(moved.columns.map((column) => [column.token, column.position])).toEqual([
      ['column-done', 0],
      ['column-backlog', 1],
      ['column-todo', 2],
    ])
  })

  it('ignores unknown columns and clamps the target', () => {
    const view = boardViewFixture()
    expect(moveColumn(view, 'missing', 0)).toBe(view)
    expect(moveColumn(view, 'column-backlog', 99).columns.map((column) => column.token)).toEqual(['column-todo', 'column-done', 'column-backlog'])
  })
})

describe('haveSameContent', () => {
  it('ignores a new position but not a changed field', () => {
    const task = taskFixture()
    expect(haveSameContent(task, task)).toBe(true)
    expect(haveSameContent(task, { ...task, position: 4 })).toBe(true)
    expect(haveSameContent(task, { ...task, title: 'Other' })).toBe(false)
    expect(haveSameContent(task, { ...task, labels: [...task.labels] })).toBe(false)
  })
})
