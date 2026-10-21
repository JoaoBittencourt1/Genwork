import type { TaskMove } from './ordering'
import type { BoardView, NewTask, Task, TaskPatch, Urgency } from './types'

export type TaskDraft = {
  title: string
  description: string
  columnToken: string
  assigneeToken: string | null
  urgency: Urgency
  labelTokens: string[]
}

export function emptyTaskDraft(columnToken: string): TaskDraft {
  return { title: '', description: '', columnToken, assigneeToken: null, urgency: 'none', labelTokens: [] }
}

export function draftFromTask(task: Task): TaskDraft {
  return {
    title: task.title,
    description: task.description ?? '',
    columnToken: task.columnToken,
    assigneeToken: task.assigneeToken,
    urgency: task.urgency,
    labelTokens: task.labels.map((label) => label.token),
  }
}

// buildTaskPatch sends only what changed, so a single edit never overwrites fields edited elsewhere (rule 17).
// The column is not part of a PATCH: resolveColumnChange turns it into a move.
export function buildTaskPatch(task: Task, draft: TaskDraft): TaskPatch {
  const original = draftFromTask(task)
  const patch: TaskPatch = {}
  const title = draft.title.trim()
  if (title !== original.title) patch.title = title
  if (draft.description !== original.description) patch.description = draft.description === '' ? null : draft.description
  if (draft.assigneeToken !== original.assigneeToken) patch.assigneeToken = draft.assigneeToken
  if (draft.urgency !== original.urgency) patch.urgency = draft.urgency
  if (!sameTokens(draft.labelTokens, original.labelTokens)) patch.labelTokens = draft.labelTokens
  return patch
}

export function isDraftTitleValid(draft: TaskDraft): boolean {
  const length = draft.title.trim().length
  return length > 0 && length <= 200
}

export function newTaskFromDraft(draft: TaskDraft): NewTask {
  const task: NewTask = { title: draft.title.trim(), columnToken: draft.columnToken, urgency: draft.urgency }
  if (draft.description !== '') task.description = draft.description
  if (draft.assigneeToken) task.assigneeToken = draft.assigneeToken
  if (draft.labelTokens.length > 0) task.labelTokens = draft.labelTokens
  return task
}

// resolveColumnChange sends a task whose column was changed in the form to the end of its new column.
export function resolveColumnChange(view: BoardView, task: Task, draft: TaskDraft): TaskMove | undefined {
  if (draft.columnToken === task.columnToken) return undefined
  const target = view.columns.find((column) => column.token === draft.columnToken)
  if (!target) return undefined
  return { taskToken: task.token, toColumnToken: target.token, toPosition: target.tasks.length }
}

function sameTokens(left: string[], right: string[]): boolean {
  return left.length === right.length && left.every((token) => right.includes(token))
}
