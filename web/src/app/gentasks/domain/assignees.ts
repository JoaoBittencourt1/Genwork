import type { DirectoryUser } from '@/shared/datasource/users'
import type { BoardView } from './types'

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
