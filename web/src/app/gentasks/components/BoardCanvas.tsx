import {
  closestCorners,
  DndContext,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import { horizontalListSortingStrategy, SortableContext, sortableKeyboardCoordinates } from '@dnd-kit/sortable'
import type { DirectoryUser } from '@/shared/datasource/users'
import { columnDragId, resolveDrop } from '../domain/dragAndDrop'
import { isSameLocation } from '../domain/ordering'
import type { GentasksAbilities } from '../domain/permissions'
import type { BoardView } from '../domain/types'
import { BoardColumn, type ColumnActions } from './BoardColumn'
import { useBoardMoves } from './useBoardMutations'

type BoardCanvasProps = {
  board: BoardView
  visibleBoard: BoardView
  abilities: GentasksAbilities
  taskDragEnabled: boolean
  users: DirectoryUser[] | undefined
  actions: ColumnActions
}

// BoardCanvas renders the visible (possibly filtered) board but resolves drops against the full board.
export function BoardCanvas({ board, visibleBoard, abilities, taskDragEnabled, users, actions }: BoardCanvasProps) {
  const moves = useBoardMoves(board.token)
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  function dropItem(event: DragEndEvent) {
    const drop = resolveDrop(board, event.active.id, event.over?.id)
    if (!drop) return
    if (drop.kind === 'column') {
      const current = board.columns.findIndex((column) => column.token === drop.columnToken)
      if (current !== drop.toPosition) moves.moveColumn({ columnToken: drop.columnToken, toPosition: drop.toPosition })
      return
    }
    if (!isSameLocation(board, drop.move)) moves.moveTask(drop.move)
  }

  return (
    <DndContext sensors={sensors} collisionDetection={closestCorners} onDragEnd={dropItem}>
      <SortableContext items={visibleBoard.columns.map((column) => columnDragId(column.token))} strategy={horizontalListSortingStrategy}>
        <div className="flex min-h-0 flex-1 items-start gap-3 overflow-x-auto pb-4">
          {visibleBoard.columns.map((column) => (
            <BoardColumn
              key={column.token}
              column={column}
              abilities={abilities}
              taskDragEnabled={taskDragEnabled}
              users={users}
              actions={actions}
            />
          ))}
        </div>
      </SortableContext>
    </DndContext>
  )
}
