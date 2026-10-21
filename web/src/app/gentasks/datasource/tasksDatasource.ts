import { apiRequest } from '@/shared/datasource/httpClient'
import type { NewTask, Task, TaskPatch } from '../domain/types'

const TASKS_PATH = '/api/gentasks/v1/tasks'

function taskPath(taskToken: string): string {
  return `${TASKS_PATH}/${encodeURIComponent(taskToken)}`
}

export function createTask(boardToken: string, task: NewTask): Promise<Task> {
  return apiRequest<Task>('POST', `/api/gentasks/v1/boards/${encodeURIComponent(boardToken)}/tasks`, { body: task })
}

export async function listArchivedTasks(boardToken: string): Promise<Task[]> {
  const path = `/api/gentasks/v1/boards/${encodeURIComponent(boardToken)}/tasks?archived=true`
  return (await apiRequest<{ tasks: Task[] }>('GET', path)).tasks
}

export function readTask(taskToken: string): Promise<Task> {
  return apiRequest<Task>('GET', taskPath(taskToken))
}

export function updateTask(taskToken: string, patch: TaskPatch): Promise<Task> {
  return apiRequest<Task>('PATCH', taskPath(taskToken), { body: patch })
}

export function moveTask(taskToken: string, columnToken: string, position: number): Promise<void> {
  return apiRequest<void>('POST', `${taskPath(taskToken)}/move`, { body: { columnToken, position } })
}

export function archiveTask(taskToken: string): Promise<void> {
  return apiRequest<void>('POST', `${taskPath(taskToken)}/archive`)
}

export function unarchiveTask(taskToken: string): Promise<void> {
  return apiRequest<void>('POST', `${taskPath(taskToken)}/unarchive`)
}

export function deleteTask(taskToken: string): Promise<void> {
  return apiRequest<void>('DELETE', taskPath(taskToken))
}
