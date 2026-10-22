import { describe, expect, it } from 'vitest'
import type { DirectoryUser } from '@/shared/datasource/users'
import { boardViewFixture } from '@/test/gentasksFixtures'
import { assignTask, canAssignFromCard } from './assignees'

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

describe('canAssignFromCard', () => {
  it('needs the permission to write tasks and the directory of users', () => {
    expect(canAssignFromCard(true, [ana])).toBe(true)
    expect(canAssignFromCard(false, [ana])).toBe(false)
    expect(canAssignFromCard(true, undefined)).toBe(false)
    expect(canAssignFromCard(true, [])).toBe(false)
  })
})
