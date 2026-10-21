import { closestCenter, closestCorners, pointerWithin, type CollisionDetection } from '@dnd-kit/core'
import { parseDragId, taskListId } from '../domain/dragAndDrop'

// boardCollisionDetection runs on every pointer move, so it narrows down before measuring: a column only meets other
// columns; a task first finds the column under the pointer, then the card under the pointer or the nearest card of
// that column, and falls back to the column itself when it is empty.
export const boardCollisionDetection: CollisionDetection = (args) => {
  const columns = args.droppableContainers.filter((container) => parseDragId(container.id)?.kind === 'column')
  if (parseDragId(args.active.id)?.kind === 'column') return closestCorners({ ...args, droppableContainers: columns })
  const [columnUnderPointer] = pointerWithin({ ...args, droppableContainers: columns })
  const column = columnUnderPointer ? parseDragId(columnUnderPointer.id) : undefined
  if (!columnUnderPointer || !column) return closestCorners({ ...args, droppableContainers: columns })
  const tasksInColumn = args.droppableContainers.filter(
    (container) => container.data.current?.sortable?.containerId === taskListId(column.token),
  )
  if (tasksInColumn.length === 0) return [columnUnderPointer]
  const taskUnderPointer = pointerWithin({ ...args, droppableContainers: tasksInColumn })
  return taskUnderPointer.length > 0 ? taskUnderPointer : closestCenter({ ...args, droppableContainers: tasksInColumn })
}
