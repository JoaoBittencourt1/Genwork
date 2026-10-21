import { HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { setAccessToken } from '@/shared/datasource/httpClient'
import { boardViewFixture, bugLabel, taskFixture } from '@/test/gentasksFixtures'
import { captureRequests, errorResponse } from '@/test/server'
import {
  addColumn,
  createBoard,
  deleteBoard,
  deleteColumn,
  listBoards,
  moveColumn,
  readBoard,
  renameBoard,
  renameColumn,
} from './boardsDatasource'
import { createLabel, deleteLabel, listLabels, updateLabel } from './labelsDatasource'
import {
  archiveTask,
  createTask,
  deleteTask,
  listArchivedTasks,
  moveTask,
  readTask,
  unarchiveTask,
  updateTask,
} from './tasksDatasource'

const noContent = () => new HttpResponse(null, { status: 204 })

describe('boardsDatasource', () => {
  it('lists, creates, reads, renames, and deletes boards', async () => {
    const board = boardViewFixture()
    captureRequests('get', '*/api/gentasks/v1/boards', () => HttpResponse.json({ boards: [board] }))
    const created = captureRequests('post', '*/api/gentasks/v1/boards', () => HttpResponse.json(board, { status: 201 }))
    const read = captureRequests('get', '*/api/gentasks/v1/boards/:token', () => HttpResponse.json(board))
    const renamed = captureRequests('patch', '*/api/gentasks/v1/boards/:token', () => HttpResponse.json(board))
    const deleted = captureRequests('delete', '*/api/gentasks/v1/boards/:token', noContent)
    setAccessToken('token-1')

    await expect(listBoards()).resolves.toEqual([board])
    await expect(createBoard('Platform')).resolves.toEqual(board)
    await expect(readBoard('board/1')).resolves.toEqual(board)
    await renameBoard('board-1', 'Renamed')
    await expect(deleteBoard('board-1')).resolves.toBeUndefined()

    expect(created[0]?.body).toEqual({ name: 'Platform' })
    expect(created[0]?.headers.get('Authorization')).toBe('Bearer token-1')
    expect(read[0]?.path).toBe('/api/gentasks/v1/boards/board%2F1')
    expect(renamed[0]?.body).toEqual({ name: 'Renamed' })
    expect(deleted[0]?.path).toBe('/api/gentasks/v1/boards/board-1')
  })

  it('manages columns', async () => {
    const column = { token: 'column-review', preset: null, name: 'Review', position: 3 }
    const added = captureRequests('post', '*/api/gentasks/v1/boards/:token/columns', () => HttpResponse.json(column, { status: 201 }))
    const renamed = captureRequests('patch', '*/api/gentasks/v1/columns/:token', () => HttpResponse.json(column))
    const moved = captureRequests('post', '*/api/gentasks/v1/columns/:token/move', noContent)
    const deleted = captureRequests('delete', '*/api/gentasks/v1/columns/:token', noContent)

    await expect(addColumn('board-1', 'Review')).resolves.toEqual(column)
    await renameColumn('column-todo', null)
    await moveColumn('column-review', 1)
    await deleteColumn('column-review')

    expect(added[0]?.body).toEqual({ name: 'Review' })
    expect(renamed[0]?.body).toEqual({ name: null })
    expect(moved[0]?.body).toEqual({ position: 1 })
    expect(deleted[0]?.path).toBe('/api/gentasks/v1/columns/column-review')
  })

  it('surfaces the column deletion conflicts', async () => {
    captureRequests('delete', '*/api/gentasks/v1/columns/:token', () => errorResponse(409, 'column_not_empty'))
    await expect(deleteColumn('column-todo')).rejects.toMatchObject({ status: 409, code: 'column_not_empty' })
  })
})

describe('labelsDatasource', () => {
  it('lists, creates, updates, and deletes labels', async () => {
    captureRequests('get', '*/api/gentasks/v1/boards/:token/labels', () => HttpResponse.json({ labels: [bugLabel] }))
    const created = captureRequests('post', '*/api/gentasks/v1/boards/:token/labels', () => HttpResponse.json(bugLabel, { status: 201 }))
    const updated = captureRequests('patch', '*/api/gentasks/v1/labels/:token', () => HttpResponse.json(bugLabel))
    const deleted = captureRequests('delete', '*/api/gentasks/v1/labels/:token', noContent)

    await expect(listLabels('board-1')).resolves.toEqual([bugLabel])
    await expect(createLabel('board-1', 'Bug', 'red')).resolves.toEqual(bugLabel)
    await updateLabel('label-bug', { color: 'blue' })
    await deleteLabel('label-bug')

    expect(created[0]?.body).toEqual({ name: 'Bug', color: 'red' })
    expect(updated[0]?.body).toEqual({ color: 'blue' })
    expect(deleted[0]?.path).toBe('/api/gentasks/v1/labels/label-bug')
  })

  it('surfaces a taken label name', async () => {
    captureRequests('post', '*/api/gentasks/v1/boards/:token/labels', () => errorResponse(409, 'label_name_taken'))
    await expect(createLabel('board-1', 'bug', 'red')).rejects.toMatchObject({ code: 'label_name_taken' })
  })
})

describe('tasksDatasource', () => {
  it('creates, reads, and patches tasks with only the given fields', async () => {
    const task = taskFixture()
    const created = captureRequests('post', '*/api/gentasks/v1/boards/:token/tasks', () => HttpResponse.json(task, { status: 201 }))
    captureRequests('get', '*/api/gentasks/v1/tasks/:token', () => HttpResponse.json(task))
    const patched = captureRequests('patch', '*/api/gentasks/v1/tasks/:token', () => HttpResponse.json(task))

    await expect(createTask('board-1', { title: 'Write tests', columnToken: 'column-todo' })).resolves.toEqual(task)
    await expect(readTask('task-1')).resolves.toEqual(task)
    await updateTask('task-1', { urgency: 'high' })

    expect(created[0]?.path).toBe('/api/gentasks/v1/boards/board-1/tasks')
    expect(created[0]?.body).toEqual({ title: 'Write tests', columnToken: 'column-todo' })
    expect(patched[0]?.body).toEqual({ urgency: 'high' })
  })

  it('moves, archives, restores, lists archived, and deletes tasks', async () => {
    const moved = captureRequests('post', '*/api/gentasks/v1/tasks/:token/move', noContent)
    const archived = captureRequests('post', '*/api/gentasks/v1/tasks/:token/archive', noContent)
    const restored = captureRequests('post', '*/api/gentasks/v1/tasks/:token/unarchive', noContent)
    const listed = captureRequests('get', '*/api/gentasks/v1/boards/:token/tasks', () =>
      HttpResponse.json({ tasks: [taskFixture({ archivedAt: '2026-10-21T13:00:00Z', position: null })] }),
    )
    const deleted = captureRequests('delete', '*/api/gentasks/v1/tasks/:token', noContent)

    await moveTask('task-1', 'column-done', 0)
    await archiveTask('task-1')
    await unarchiveTask('task-1')
    await expect(listArchivedTasks('board-1')).resolves.toHaveLength(1)
    await deleteTask('task-1')

    expect(moved[0]?.body).toEqual({ columnToken: 'column-done', position: 0 })
    expect(archived[0]?.path).toBe('/api/gentasks/v1/tasks/task-1/archive')
    expect(restored[0]?.path).toBe('/api/gentasks/v1/tasks/task-1/unarchive')
    expect(listed[0]?.search).toBe('?archived=true')
    expect(deleted[0]?.method).toBe('DELETE')
  })

  it('surfaces archived-task and core errors', async () => {
    captureRequests('post', '*/api/gentasks/v1/tasks/:token/move', () => errorResponse(409, 'task_archived'))
    captureRequests('post', '*/api/gentasks/v1/boards/:token/tasks', () => errorResponse(503, 'core_unavailable'))

    await expect(moveTask('task-1', 'column-done', 0)).rejects.toMatchObject({ code: 'task_archived' })
    await expect(createTask('board-1', { title: 't', assigneeToken: 'user-ana' })).rejects.toMatchObject({ status: 503 })
  })
})
