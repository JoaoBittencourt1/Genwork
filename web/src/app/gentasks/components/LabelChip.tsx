import { labelColorVariable } from '../domain/labelPalette'
import type { Label } from '../domain/types'

export function LabelChip({ label }: { label: Label }) {
  return (
    <span className="inline-flex max-w-full items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs">
      <span className="size-2 shrink-0 rounded-full" style={{ backgroundColor: labelColorVariable(label.color) }} />
      <span className="truncate">{label.name}</span>
    </span>
  )
}
