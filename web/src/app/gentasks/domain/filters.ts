import type { BoardView, Task, Urgency } from './types'

// UNASSIGNED stands for "tasks without an assignee" among the selected assignees.
export const UNASSIGNED = 'unassigned'

export type TaskFilter = {
  urgencies: Urgency[]
  labelTokens: string[]
  assigneeTokens: string[]
}

export const EMPTY_FILTER: TaskFilter = { urgencies: [], labelTokens: [], assigneeTokens: [] }

export function isFilterActive(filter: TaskFilter): boolean {
  return filter.urgencies.length > 0 || filter.labelTokens.length > 0 || filter.assigneeTokens.length > 0
}

// matchesFilter combines dimensions with AND and the values of one dimension with OR; an empty dimension matches all.
export function matchesFilter(task: Task, filter: TaskFilter): boolean {
  const urgencyMatches = filter.urgencies.length === 0 || filter.urgencies.includes(task.urgency)
  const labelMatches =
    filter.labelTokens.length === 0 || task.labels.some((label) => filter.labelTokens.includes(label.token))
  const assigneeMatches =
    filter.assigneeTokens.length === 0 || filter.assigneeTokens.includes(task.assigneeToken ?? UNASSIGNED)
  return urgencyMatches && labelMatches && assigneeMatches
}

// filterBoardView only hides cards; it never changes their stored order.
export function filterBoardView(view: BoardView, filter: TaskFilter): BoardView {
  if (!isFilterActive(filter)) return view
  return {
    ...view,
    columns: view.columns.map((column) => ({ ...column, tasks: column.tasks.filter((task) => matchesFilter(task, filter)) })),
  }
}

export function toggleValue<T>(values: T[], value: T): T[] {
  return values.includes(value) ? values.filter((candidate) => candidate !== value) : [...values, value]
}
