import {
  DndContext,
  DragOverlay,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragOverEvent,
  type DragStartEvent,
} from '@dnd-kit/core'
import { horizontalListSortingStrategy, SortableContext, sortableKeyboardCoordinates } from '@dnd-kit/sortable'
import { useState } from 'react'
import type { DirectoryUser } from '@/shared/datasource/users'
import { columnDragId, parseDragId, resolveDragOver, resolveDrop } from '../domain/dragAndDrop'
import { isSameLocation, moveColumn, moveTask } from '../domain/ordering'
import type { GentasksAbilities } from '../domain/permissions'
import type { BoardView } from '../domain/types'
import { BoardColumn, type ColumnActions } from './BoardColumn'
import { boardCollisionDetection } from './boardCollisionDetection'
import { TaskCardPreview } from './TaskCard'
import { useBoardMoves } from './useBoardMutations'

type BoardCanvasProps = {
  board: BoardView
  visibleBoard: BoardView
  abilities: GentasksAbilities
  taskDragEnabled: boolean
  users: DirectoryUser[] | undefined
  actions: ColumnActions
}

// TaskDrag is the board as it looks while a card is held: the card already sits in the column under the pointer.
type TaskDrag = { taskToken: string; view: BoardView }

// DroppedBoard is the board as it was dropped. It is shown until the cached board first changes (usually the optimistic
// update), so a card never flashes back to its old column, nor does the drop animation head there. It is cleared on that
// first change, so a rollback to the very same board object shows the old order again.
type DroppedBoard = { view: BoardView; basedOn: BoardView }

// BoardCanvas renders the visible (possibly filtered) board but resolves drops against the full board.
export function BoardCanvas({ board, visibleBoard, abilities, taskDragEnabled, users, actions }: BoardCanvasProps) {
  const moves = useBoardMoves(board.token)
  const [taskDrag, setTaskDrag] = useState<TaskDrag | null>(null)
  const [dropped, setDropped] = useState<DroppedBoard | null>(null)
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )
  if (dropped && dropped.basedOn !== board) setDropped(null)
  const shownBoard = taskDrag?.view ?? (dropped?.basedOn === board ? dropped.view : visibleBoard)
  const draggedTask = taskDrag?.view.columns.flatMap((column) => column.tasks).find((task) => task.token === taskDrag.taskToken)

  function startDrag(event: DragStartEvent) {
    const dragged = parseDragId(event.active.id)
    if (dragged?.kind === 'task') setTaskDrag({ taskToken: dragged.token, view: board })
  }

  function dragOver(event: DragOverEvent) {
    setTaskDrag((current) => {
      const move = current ? resolveDragOver(current.view, event.active.id, event.over?.id) : undefined
      return current && move ? { ...current, view: moveTask(current.view, move) } : current
    })
  }

  function dropItem(event: DragEndEvent) {
    const current = taskDrag?.view ?? board
    setTaskDrag(null)
    const drop = resolveDrop(current, event.active.id, event.over?.id)
    if (!drop) return
    if (drop.kind === 'column') {
      const position = board.columns.findIndex((column) => column.token === drop.columnToken)
      if (position === drop.toPosition) return
      setDropped({ view: moveColumn(board, drop.columnToken, drop.toPosition), basedOn: board })
      moves.moveColumn({ columnToken: drop.columnToken, toPosition: drop.toPosition })
      return
    }
    if (isSameLocation(board, drop.move)) return
    setDropped({ view: moveTask(board, drop.move), basedOn: board })
    moves.moveTask(drop.move)
  }

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={boardCollisionDetection}
      onDragStart={startDrag}
      onDragOver={dragOver}
      onDragEnd={dropItem}
      onDragCancel={() => setTaskDrag(null)}
    >
      <SortableContext items={shownBoard.columns.map((column) => columnDragId(column.token))} strategy={horizontalListSortingStrategy}>
        <div className="flex min-h-0 flex-1 items-stretch gap-3 overflow-x-auto p-3">
          {shownBoard.columns.map((column) => (
            <BoardColumn
              key={column.token}
              column={column}
              taskPrefix={board.taskPrefix}
              abilities={abilities}
              taskDragEnabled={taskDragEnabled}
              users={users}
              actions={actions}
            />
          ))}
        </div>
      </SortableContext>
      <DragOverlay>{draggedTask ? <TaskCardPreview task={draggedTask} taskPrefix={board.taskPrefix} users={users} /> : null}</DragOverlay>
    </DndContext>
  )
}
