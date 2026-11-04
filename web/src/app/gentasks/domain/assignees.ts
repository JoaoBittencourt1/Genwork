import type { DirectoryUser } from '@/shared/datasource/users'
import type { BoardView } from './types'

export type AssigneeTaskCount = {
  assigneeToken: string | null
  user: DirectoryUser | undefined
  count: number
  percentage: number
}

export type BoardAssigneeBreakdown = {
  totalTasks: number
  entries: AssigneeTaskCount[]
}

export type BoardProgressPoint = {
  step: number
  completedCount: number
  percentage: number
  taskNumber: number | null
  title: string | null
}

export const DISTINCT_CHART_COLORS = [
  'var(--label-blue)',
  'var(--label-green)',
  'var(--label-orange)',
  'var(--label-purple)',
  'var(--label-pink)',
  'var(--label-teal)',
  'var(--label-yellow)',
  'var(--label-red)',
  'var(--avatar-blue)',
  'var(--avatar-light-green)',
  'var(--avatar-orange)',
  'var(--avatar-purple)',
  'var(--avatar-pink)',
  'var(--avatar-yellow)',
  'var(--avatar-red)',
  'var(--label-gray)',
] as const

export type AssigneeProgressSeries = {
  assigneeToken: string | null
  user: DirectoryUser | undefined
  color: string
  completedTasks: number
  completionPercentage: number
  points: BoardProgressPoint[]
}

export type BoardProgressSummary = {
  totalTasks: number
  completedTasks: number
  completionPercentage: number
  maxY: number
  points: BoardProgressPoint[]
  assigneeSeries: AssigneeProgressSeries[]
}

export function findAssignee(users: DirectoryUser[] | undefined, assigneeToken: string | null): DirectoryUser | undefined {
  if (!assigneeToken) return undefined
  return users?.find((user) => user.token === assigneeToken)
}

// assignTask mirrors a task's new assignee on the board, so the card changes before the request answers.
// Tasks it does not touch keep their identity, so their memoized cards do not render again.
export function assignTask(view: BoardView, taskToken: string, assigneeToken: string | null): BoardView {
  let found = false
  const columns = view.columns.map((column) => {
    if (!column.tasks.some((task) => task.token === taskToken)) return column
    found = true
    return { ...column, tasks: column.tasks.map((task) => (task.token === taskToken ? { ...task, assigneeToken } : task)) }
  })
  return found ? { ...view, columns } : view
}

// canAssignFromCard hides the card's assignee menu from people who cannot write tasks, and while there is nobody to pick.
export function canAssignFromCard(canWriteTasks: boolean, users: DirectoryUser[] | undefined): boolean {
  return canWriteTasks && users !== undefined && users.length > 0
}

export function summarizeBoardAssignees(view: BoardView, users: DirectoryUser[] | undefined): BoardAssigneeBreakdown {
  const countsByToken = new Map<string, number>()
  let unassignedCount = 0
  let totalTasks = 0

  for (const column of view.columns) {
    for (const task of column.tasks) {
      totalTasks += 1
      if (task.assigneeToken === null) {
        unassignedCount += 1
      } else {
        countsByToken.set(task.assigneeToken, (countsByToken.get(task.assigneeToken) ?? 0) + 1)
      }
    }
  }

  const toPercentage = (count: number) => (totalTasks === 0 ? 0 : Math.round((count / totalTasks) * 100))

  const memberEntries: AssigneeTaskCount[] = (users ?? [])
    .map((user) => {
      const count = countsByToken.get(user.token) ?? 0
      return {
        assigneeToken: user.token,
        user,
        count,
        percentage: toPercentage(count),
      }
    })
    .sort((left, right) => right.count - left.count || (left.user?.name ?? '').localeCompare(right.user?.name ?? ''))

  const unassignedEntry: AssigneeTaskCount = {
    assigneeToken: null,
    user: undefined,
    count: unassignedCount,
    percentage: toPercentage(unassignedCount),
  }

  return {
    totalTasks,
    entries: [...memberEntries, unassignedEntry],
  }
}

export function summarizeBoardProgress(view: BoardView, users?: DirectoryUser[]): BoardProgressSummary {
  const totalTasks = view.columns.reduce((sum, column) => sum + column.tasks.length, 0)
  const doneColumn = view.columns.find((column) => column.preset === 'done') ?? view.columns.at(-1)
  const doneTasks = [...(doneColumn?.tasks ?? [])].sort(
    (left, right) => Date.parse(left.updatedAt) - Date.parse(right.updatedAt) || left.number - right.number,
  )

  const toPercentage = (count: number) => (totalTasks === 0 ? 0 : Math.round((count / totalTasks) * 100))
  const completedTasks = doneTasks.length
  const completionPercentage = toPercentage(completedTasks)
  const maxY = Math.max(totalTasks, 1)

  const points: BoardProgressPoint[] = [
    { step: 0, completedCount: 0, percentage: 0, taskNumber: null, title: null },
    ...doneTasks.map((task, index) => ({
      step: index + 1,
      completedCount: index + 1,
      percentage: toPercentage(index + 1),
      taskNumber: task.number,
      title: task.title,
    })),
  ]

  const buildSeriesFor = (assigneeToken: string | null, user: DirectoryUser | undefined): AssigneeProgressSeries => {
    let runningCompleted = 0
    const seriesPoints: BoardProgressPoint[] = [
      { step: 0, completedCount: 0, percentage: 0, taskNumber: null, title: null },
    ]
    for (const [index, task] of doneTasks.entries()) {
      const matches = task.assigneeToken === assigneeToken
      if (matches) {
        runningCompleted += 1
      }
      seriesPoints.push({
        step: index + 1,
        completedCount: runningCompleted,
        percentage: toPercentage(runningCompleted),
        taskNumber: matches ? task.number : null,
        title: matches ? task.title : null,
      })
    }
    return {
      assigneeToken,
      user,
      color: DISTINCT_CHART_COLORS[0],
      completedTasks: runningCompleted,
      completionPercentage: toPercentage(runningCompleted),
      points: seriesPoints,
    }
  }

  const assigneeSeries: AssigneeProgressSeries[] = (users ?? [])
    .map((user) => buildSeriesFor(user.token, user))
    .sort(
      (left, right) =>
        right.completedTasks - left.completedTasks || (left.user?.name ?? '').localeCompare(right.user?.name ?? ''),
    )
    .map((series, index) => ({
      ...series,
      color: DISTINCT_CHART_COLORS[index % DISTINCT_CHART_COLORS.length] ?? DISTINCT_CHART_COLORS[0],
    }))

  return {
    totalTasks,
    completedTasks,
    completionPercentage,
    maxY,
    points,
    assigneeSeries,
  }
}
