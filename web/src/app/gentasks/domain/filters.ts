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

export function isAssigneeSelected(filter: TaskFilter, assigneeToken: string | null): boolean {
  return filter.assigneeTokens.includes(assigneeToken ?? UNASSIGNED)
}

export function toggleAssigneeFilter(filter: TaskFilter, assigneeToken: string | null): TaskFilter {
  return {
    ...filter,
    assigneeTokens: toggleValue(filter.assigneeTokens, assigneeToken ?? UNASSIGNED),
  }
}

// selectedNames lists the names of the selected options in selection order; unknown tokens show as-is.
export function selectedNames(tokens: string[], options: { token: string; name: string }[]): string {
  return tokens.map((token) => options.find((option) => option.token === token)?.name ?? token).join(', ')
}
