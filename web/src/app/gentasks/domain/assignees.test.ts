import { describe, expect, it } from 'vitest'
import type { DirectoryUser } from '@/shared/datasource/users'
import { boardTaskFixture, boardViewFixture, usersFixture } from '@/test/gentasksFixtures'
import { assignTask, canAssignFromCard, summarizeBoardAssignees, summarizeBoardProgress } from './assignees'

function assigneeOf(view: ReturnType<typeof boardViewFixture>, taskToken: string) {
  return view.columns.flatMap((column) => column.tasks).find((task) => task.token === taskToken)?.assigneeToken
}

describe('assignTask', () => {
  it('sets the assignee of one task and leaves every other task as it was', () => {
    const view = boardViewFixture()

    const assigned = assignTask(view, 'task-B', 'user-bruno')

    expect(assigneeOf(assigned, 'task-B')).toBe('user-bruno')
    expect(assigneeOf(assigned, 'task-A')).toBe(assigneeOf(view, 'task-A'))
    expect(assigned.columns[0]?.tasks[0]).toBe(view.columns[0]?.tasks[0])
    expect(assigneeOf(view, 'task-B')).toBeNull()
  })

  it('clears the assignee', () => {
    const assigned = assignTask(boardViewFixture(), 'task-B', 'user-bruno')

    expect(assigneeOf(assignTask(assigned, 'task-B', null), 'task-B')).toBeNull()
  })

  it('returns the same view for an unknown task', () => {
    const view = boardViewFixture()

    expect(assignTask(view, 'missing', 'user-bruno')).toBe(view)
  })
})

const ana: DirectoryUser = { token: 'user-ana', name: 'Ana', email: 'ana@example.com', avatarColor: 'blue' }
const carla: DirectoryUser = { token: 'user-carla', name: 'Carla', email: 'carla@example.com', avatarColor: 'light-green' }

describe('canAssignFromCard', () => {
  it('needs the permission to write tasks and the directory of users', () => {
    expect(canAssignFromCard(true, [ana])).toBe(true)
    expect(canAssignFromCard(false, [ana])).toBe(false)
    expect(canAssignFromCard(true, undefined)).toBe(false)
    expect(canAssignFromCard(true, [])).toBe(false)
  })
})

describe('summarizeBoardAssignees', () => {
  it('counts tasks per team member sorted by count then name and includes unassigned at the end', () => {
    let view = boardViewFixture()
    view = assignTask(view, 'task-A', 'user-bruno')
    view = assignTask(view, 'task-B', 'user-bruno')
    view = assignTask(view, 'task-D', 'user-ana')

    const breakdown = summarizeBoardAssignees(view, [...usersFixture, carla])

    expect(breakdown.totalTasks).toBe(4)
    expect(breakdown.entries).toEqual([
      { assigneeToken: 'user-bruno', user: usersFixture[1], count: 2, percentage: 50 },
      { assigneeToken: 'user-ana', user: usersFixture[0], count: 1, percentage: 25 },
      { assigneeToken: 'user-carla', user: carla, count: 0, percentage: 0 },
      { assigneeToken: null, user: undefined, count: 1, percentage: 25 },
    ])
  })

  it('returns zero percentages when the board has no tasks', () => {
    const emptyBoard = {
      ...boardViewFixture(),
      columns: boardViewFixture().columns.map((column) => ({ ...column, tasks: [] })),
    }

    const breakdown = summarizeBoardAssignees(emptyBoard, usersFixture)

    expect(breakdown.totalTasks).toBe(0)
    expect(breakdown.entries).toEqual([
      { assigneeToken: 'user-ana', user: usersFixture[0], count: 0, percentage: 0 },
      { assigneeToken: 'user-bruno', user: usersFixture[1], count: 0, percentage: 0 },
      { assigneeToken: null, user: undefined, count: 0, percentage: 0 },
    ])
  })

  it('handles an unavailable user directory', () => {
    const view = assignTask(boardViewFixture(), 'task-A', 'user-ana')

    const breakdown = summarizeBoardAssignees(view, undefined)

    expect(breakdown.totalTasks).toBe(4)
    expect(breakdown.entries).toEqual([{ assigneeToken: null, user: undefined, count: 3, percentage: 75 }])
  })
})

describe('summarizeBoardProgress', () => {
  it('builds cumulative cartesian points for project and per-member lines ordered by completion time', () => {
    const base = boardViewFixture()
    const view = {
      ...base,
      columns: base.columns.map((column) =>
        column.preset === 'done'
          ? {
              ...column,
              tasks: [
                boardTaskFixture({
                  token: 'task-E',
                  number: 5,
                  title: 'Ship API',
                  columnToken: column.token,
                  assigneeToken: 'user-ana',
                  updatedAt: '2026-10-22T15:00:00Z',
                }),
                boardTaskFixture({
                  token: 'task-F',
                  number: 6,
                  title: 'Design DB',
                  columnToken: column.token,
                  assigneeToken: 'user-bruno',
                  updatedAt: '2026-10-22T10:00:00Z',
                }),
              ],
            }
          : column,
      ),
    }

    const progress = summarizeBoardProgress(view, usersFixture)

    expect(progress.totalTasks).toBe(6)
    expect(progress.completedTasks).toBe(2)
    expect(progress.completionPercentage).toBe(33)
    expect(progress.maxY).toBe(6)
    expect(progress.points).toEqual([
      { step: 0, completedCount: 0, percentage: 0, taskNumber: null, title: null },
      { step: 1, completedCount: 1, percentage: 17, taskNumber: 6, title: 'Design DB' },
      { step: 2, completedCount: 2, percentage: 33, taskNumber: 5, title: 'Ship API' },
    ])
    expect(progress.assigneeSeries).toEqual([
      {
        assigneeToken: 'user-ana',
        user: usersFixture[0],
        color: 'var(--label-blue)',
        completedTasks: 1,
        completionPercentage: 17,
        points: [
          { step: 0, completedCount: 0, percentage: 0, taskNumber: null, title: null },
          { step: 1, completedCount: 0, percentage: 0, taskNumber: null, title: null },
          { step: 2, completedCount: 1, percentage: 17, taskNumber: 5, title: 'Ship API' },
        ],
      },
      {
        assigneeToken: 'user-bruno',
        user: usersFixture[1],
        color: 'var(--label-green)',
        completedTasks: 1,
        completionPercentage: 17,
        points: [
          { step: 0, completedCount: 0, percentage: 0, taskNumber: null, title: null },
          { step: 1, completedCount: 1, percentage: 17, taskNumber: 6, title: 'Design DB' },
          { step: 2, completedCount: 1, percentage: 17, taskNumber: null, title: null },
        ],
      },
    ])
  })

  it('assigns distinct chart colors even when team members share the same avatarColor', () => {
    const sameAvatarUsers: DirectoryUser[] = [
      { token: 'user-1', name: 'Alice', email: 'alice@example.com', avatarColor: 'blue' },
      { token: 'user-2', name: 'Bob', email: 'bob@example.com', avatarColor: 'blue' },
      { token: 'user-3', name: 'Carol', email: 'carol@example.com', avatarColor: 'blue' },
    ]

    const progress = summarizeBoardProgress(boardViewFixture(), sameAvatarUsers)
    const colors = progress.assigneeSeries.map((series) => series.color)

    expect(new Set(colors).size).toBe(3)
  })

  it('falls back to the last column when no preset done column exists and handles empty boards', () => {
    const emptyBoard = {
      ...boardViewFixture(),
      columns: [
        { token: 'col-1', preset: null, name: 'Etapa 1', position: 0, tasks: [] },
        { token: 'col-2', preset: null, name: 'Final', position: 1, tasks: [] },
      ],
    }

    const progress = summarizeBoardProgress(emptyBoard)

    expect(progress).toEqual({
      totalTasks: 0,
      completedTasks: 0,
      completionPercentage: 0,
      maxY: 1,
      points: [{ step: 0, completedCount: 0, percentage: 0, taskNumber: null, title: null }],
      assigneeSeries: [],
    })
  })
})
