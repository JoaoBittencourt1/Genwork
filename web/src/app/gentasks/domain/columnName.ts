import type { Column } from './types'

export type ColumnTitle = { kind: 'translated'; key: string } | { kind: 'custom'; name: string }

// columnTitle prefers a custom name; a preset column without one shows its localized preset label.
export function columnTitle(column: Pick<Column, 'preset' | 'name'>): ColumnTitle {
  if (column.name !== null) {
    return { kind: 'custom', name: column.name }
  }
  return { kind: 'translated', key: `gentasks.columnPresets.${column.preset ?? 'backlog'}` }
}

export function resolveColumnTitle(column: Pick<Column, 'preset' | 'name'>, translate: (key: string) => string): string {
  const title = columnTitle(column)
  return title.kind === 'custom' ? title.name : translate(title.key)
}

// canClearColumnName: only preset columns can fall back to their preset label.
export function canClearColumnName(column: Pick<Column, 'preset' | 'name'>): boolean {
  return column.preset !== null && column.name !== null
}
