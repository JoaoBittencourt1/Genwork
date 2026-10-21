import { isFilterActive, type TaskFilter } from './filters'
import type { TaskMove } from './ordering'
import type { BoardView } from './types'

const TASK_PREFIX = 'task:'
const COLUMN_PREFIX = 'column:'

export type DragItem = { kind: 'task'; token: string } | { kind: 'column'; token: string }

export type Drop =
  | { kind: 'task'; move: TaskMove }
  | { kind: 'column'; columnToken: string; toPosition: number }

export function taskDragId(token: string): string {
  return TASK_PREFIX + token
}

export function columnDragId(token: string): string {
  return COLUMN_PREFIX + token
}

export function parseDragId(id: string | number): DragItem | undefined {
  const value = String(id)
  if (value.startsWith(TASK_PREFIX)) return { kind: 'task', token: value.slice(TASK_PREFIX.length) }
  if (value.startsWith(COLUMN_PREFIX)) return { kind: 'column', token: value.slice(COLUMN_PREFIX.length) }
  return undefined
}

// resolveDrop turns "dragged X over Y" into the move the API expects:
// a task dropped on a task takes its place, a task dropped on a column goes last, a column takes the target column's place.
export function resolveDrop(view: BoardView, activeId: string | number, overId: string | number | undefined): Drop | undefined {
  const active = parseDragId(activeId)
  const over = overId === undefined ? undefined : parseDragId(overId)
  if (!active || !over) return undefined
  if (active.kind === 'column') {
    const columnToken = over.kind === 'column' ? over.token : columnOfTask(view, over.token)
    const toPosition = view.columns.findIndex((column) => column.token === columnToken)
    return toPosition < 0 ? undefined : { kind: 'column', columnToken: active.token, toPosition }
  }
  if (over.kind === 'column') {
    const column = view.columns.find((candidate) => candidate.token === over.token)
    if (!column) return undefined
    const othersInColumn = column.tasks.filter((task) => task.token !== active.token).length
    return { kind: 'task', move: { taskToken: active.token, toColumnToken: column.token, toPosition: othersInColumn } }
  }
  const toColumnToken = columnOfTask(view, over.token)
  const column = view.columns.find((candidate) => candidate.token === toColumnToken)
  if (!column) return undefined
  const toPosition = column.tasks.findIndex((task) => task.token === over.token)
  return { kind: 'task', move: { taskToken: active.token, toColumnToken: column.token, toPosition } }
}

// Dragging tasks while a filter hides some of them would compute positions against an incomplete column.
export function isTaskDragEnabled(canWriteTasks: boolean, filter: TaskFilter): boolean {
  return canWriteTasks && !isFilterActive(filter)
}

function columnOfTask(view: BoardView, taskToken: string): string | undefined {
  return view.columns.find((column) => column.tasks.some((task) => task.token === taskToken))?.token
}
