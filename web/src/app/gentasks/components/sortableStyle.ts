import type { CSSProperties } from 'react'

type SortableState = {
  transform: { x: number; y: number } | null
  transition: string | undefined
  isDragging: boolean
}

export function sortableStyle({ transform, transition, isDragging }: SortableState): CSSProperties {
  return {
    transform: transform ? `translate3d(${transform.x}px, ${transform.y}px, 0)` : undefined,
    transition,
    opacity: isDragging ? 0.5 : undefined,
  }
}
