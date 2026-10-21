import { describe, expect, it } from 'vitest'
import { boardViewFixture } from '@/test/gentasksFixtures'
import { columnDragId, isTaskDragEnabled, parseDragId, resolveDragOver, resolveDrop, taskDragId, taskListId } from './dragAndDrop'
import { EMPTY_FILTER } from './filters'

describe('drag ids', () => {
  it('round-trips task and column ids and rejects others', () => {
    expect(parseDragId(taskDragId('task-A'))).toEqual({ kind: 'task', token: 'task-A' })
    expect(parseDragId(columnDragId('column-todo'))).toEqual({ kind: 'column', token: 'column-todo' })
    expect(parseDragId('something')).toBeUndefined()
    expect(parseDragId(taskListId('column-todo'))).toBeUndefined()
  })
})

describe('resolveDrop', () => {
  const view = boardViewFixture()

  it('puts a task in the place of the task it is dropped on', () => {
    expect(resolveDrop(view, taskDragId('task-A'), taskDragId('task-D'))).toEqual({
      kind: 'task',
      move: { taskToken: 'task-A', toColumnToken: 'column-todo', toPosition: 0 },
    })
    expect(resolveDrop(view, taskDragId('task-A'), taskDragId('task-C'))).toEqual({
      kind: 'task',
      move: { taskToken: 'task-A', toColumnToken: 'column-backlog', toPosition: 2 },
    })
  })

  it('appends a task dropped on a column', () => {
    expect(resolveDrop(view, taskDragId('task-A'), columnDragId('column-done'))).toEqual({
      kind: 'task',
      move: { taskToken: 'task-A', toColumnToken: 'column-done', toPosition: 0 },
    })
    expect(resolveDrop(view, taskDragId('task-A'), columnDragId('column-backlog'))).toEqual({
      kind: 'task',
      move: { taskToken: 'task-A', toColumnToken: 'column-backlog', toPosition: 2 },
    })
  })

  it('moves a column to the place of the column or task it is dropped on', () => {
    expect(resolveDrop(view, columnDragId('column-done'), columnDragId('column-backlog'))).toEqual({
      kind: 'column',
      columnToken: 'column-done',
      toPosition: 0,
    })
    expect(resolveDrop(view, columnDragId('column-backlog'), taskDragId('task-D'))).toEqual({
      kind: 'column',
      columnToken: 'column-backlog',
      toPosition: 1,
    })
  })

  it('ignores drops outside the board', () => {
    expect(resolveDrop(view, taskDragId('task-A'), undefined)).toBeUndefined()
    expect(resolveDrop(view, 'other', taskDragId('task-A'))).toBeUndefined()
    expect(resolveDrop(view, taskDragId('task-A'), columnDragId('missing'))).toBeUndefined()
    expect(resolveDrop(view, taskDragId('task-A'), taskDragId('missing'))).toBeUndefined()
    expect(resolveDrop(view, columnDragId('column-todo'), taskDragId('missing'))).toBeUndefined()
  })
})

describe('resolveDragOver', () => {
  const view = boardViewFixture()

  it('moves a task into the column it is dragged over, in the place of the hovered task', () => {
    expect(resolveDragOver(view, taskDragId('task-A'), taskDragId('task-D'))).toEqual({
      taskToken: 'task-A',
      toColumnToken: 'column-todo',
      toPosition: 0,
    })
  })

  it('appends a task dragged over the empty part of another column', () => {
    expect(resolveDragOver(view, taskDragId('task-D'), columnDragId('column-backlog'))).toEqual({
      taskToken: 'task-D',
      toColumnToken: 'column-backlog',
      toPosition: 3,
    })
  })

  it('leaves reordering within a column to the sortable list', () => {
    expect(resolveDragOver(view, taskDragId('task-A'), taskDragId('task-C'))).toBeUndefined()
    expect(resolveDragOver(view, taskDragId('task-A'), columnDragId('column-backlog'))).toBeUndefined()
  })

  it('ignores columns being dragged and unknown targets', () => {
    expect(resolveDragOver(view, columnDragId('column-todo'), columnDragId('column-done'))).toBeUndefined()
    expect(resolveDragOver(view, taskDragId('task-A'), undefined)).toBeUndefined()
    expect(resolveDragOver(view, taskDragId('task-A'), columnDragId('missing'))).toBeUndefined()
    expect(resolveDragOver(view, taskDragId('missing'), columnDragId('column-done'))).toBeUndefined()
  })
})

describe('isTaskDragEnabled', () => {
  it('needs the write permission and no active filter', () => {
    expect(isTaskDragEnabled(true, EMPTY_FILTER)).toBe(true)
    expect(isTaskDragEnabled(false, EMPTY_FILTER)).toBe(false)
    expect(isTaskDragEnabled(true, { ...EMPTY_FILTER, urgencies: ['high'] })).toBe(false)
  })
})
