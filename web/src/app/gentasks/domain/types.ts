export const URGENCY_LEVELS = ['none', 'low', 'medium', 'high', 'urgent'] as const
export type Urgency = (typeof URGENCY_LEVELS)[number]

export const LABEL_COLORS = ['gray', 'red', 'orange', 'yellow', 'green', 'teal', 'blue', 'purple', 'pink'] as const
export type LabelColor = (typeof LABEL_COLORS)[number]

export const COLUMN_PRESETS = ['backlog', 'todo', 'done'] as const
export type ColumnPreset = (typeof COLUMN_PRESETS)[number]

export type BoardSummary = {
  token: string
  name: string
  createdBy: string
  createdAt: string
  updatedAt: string
}

export type Column = {
  token: string
  preset: ColumnPreset | null
  name: string | null
  position: number
}

export type Label = {
  token: string
  name: string
  color: LabelColor
}

export type Task = {
  token: string
  columnToken: string
  position: number | null
  title: string
  description: string | null
  assigneeToken: string | null
  urgency: Urgency
  labels: Label[]
  createdBy: string
  archivedAt: string | null
  createdAt: string
  updatedAt: string
}

export type ColumnWithTasks = Column & { tasks: Task[] }

export type BoardView = BoardSummary & {
  columns: ColumnWithTasks[]
  labels: Label[]
}

export type CreatedBoard = BoardSummary & { columns: Column[] }

export type NewTask = {
  title: string
  description?: string
  columnToken?: string
  assigneeToken?: string
  labelTokens?: string[]
  urgency?: Urgency
}

// TaskPatch follows the API's PATCH rules: an omitted key leaves the field unchanged, null clears it.
export type TaskPatch = {
  title?: string
  description?: string | null
  assigneeToken?: string | null
  urgency?: Urgency
  labelTokens?: string[]
}

export type LabelPatch = {
  name?: string
  color?: LabelColor
}
