import type { BoardView, ColumnWithTasks, Task } from './types'

export type TaskLocation = { columnToken: string; index: number }

export type TaskMove = { taskToken: string; toColumnToken: string; toPosition: number }

export function findTaskLocation(view: BoardView, taskToken: string): TaskLocation | undefined {
  for (const column of view.columns) {
    const index = column.tasks.findIndex((task) => task.token === taskToken)
    if (index >= 0) return { columnToken: column.token, index }
  }
  return undefined
}

// moveTask mirrors the server's ordering rules, so the board can update before the request answers;
// keeping the previous view is enough to roll back.
export function moveTask(view: BoardView, move: TaskMove): BoardView {
  const location = findTaskLocation(view, move.taskToken)
  const target = view.columns.find((column) => column.token === move.toColumnToken)
  if (!location || !target) return view
  const source = view.columns.find((column) => column.token === location.columnToken) as ColumnWithTasks
  const moving = source.tasks[location.index] as Task

  const columns = view.columns.map((column) => {
    let tasks = column.tasks.filter((task) => task.token !== move.taskToken)
    if (column.token === target.token) {
      const position = clamp(move.toPosition, tasks.length)
      tasks = [...tasks.slice(0, position), { ...moving, columnToken: target.token }, ...tasks.slice(position)]
    }
    return column.token === source.token || column.token === target.token ? { ...column, tasks: renumberTasks(tasks) } : column
  })
  return { ...view, columns }
}

export function moveColumn(view: BoardView, columnToken: string, toPosition: number): BoardView {
  const moving = view.columns.find((column) => column.token === columnToken)
  if (!moving) return view
  const rest = view.columns.filter((column) => column.token !== columnToken)
  const position = clamp(toPosition, rest.length)
  const columns = [...rest.slice(0, position), moving, ...rest.slice(position)]
  return { ...view, columns: columns.map((column, index) => ({ ...column, position: index })) }
}

export function isSameLocation(view: BoardView, move: TaskMove): boolean {
  const location = findTaskLocation(view, move.taskToken)
  return location?.columnToken === move.toColumnToken && location.index === move.toPosition
}

function renumberTasks(tasks: Task[]): Task[] {
  return tasks.map((task, index) => (task.position === index ? task : { ...task, position: index }))
}

function clamp(position: number, last: number): number {
  return Math.max(0, Math.min(position, last))
}
