export const URGENCY_LEVELS = ['none', 'low', 'medium', 'high', 'urgent'] as const
export type Urgency = (typeof URGENCY_LEVELS)[number]

export const LABEL_COLORS = ['gray', 'red', 'orange', 'yellow', 'green', 'teal', 'blue', 'purple', 'pink'] as const
export type LabelColor = (typeof LABEL_COLORS)[number]

export const COLUMN_PRESETS = ['backlog', 'todo', 'done'] as const
export type ColumnPreset = (typeof COLUMN_PRESETS)[number]

export type BoardSummary = {
  token: string
  name: string
  taskPrefix: string
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
  number: number
  columnToken: string
  position: number | null
  title: string
  description: string | null
  assigneeToken: string | null
  urgency: Urgency
  labels: Label[]
  createdBy: string
  assignedAt: string | null
  archivedAt: string | null
  createdAt: string
  updatedAt: string
}

export const PULL_REQUEST_STATES = ['open', 'draft', 'merged', 'closed'] as const
export type PullRequestState = (typeof PULL_REQUEST_STATES)[number]

// TaskPullRequest: title, state, and author are null for pull requests linked before gentasks kept their details.
export type TaskPullRequest = {
  token: string
  repository: string
  number: number
  title: string | null
  state: PullRequestState | null
  author: { login: string; avatarUrl: string } | null
  url: string
}

// BoardTask is a task as the board view returns it, with the pull requests that cite it.
export type BoardTask = Task & { pullRequests: TaskPullRequest[] }

// AssignedTask is a task from the caller's assigned-tasks list, with the board and column it sits in.
export type AssignedTask = Task & {
  board: Pick<BoardSummary, 'token' | 'name' | 'taskPrefix'>
  column: Pick<Column, 'token' | 'preset' | 'name'>
}

export type ColumnWithTasks = Column & { tasks: BoardTask[] }

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

// BoardPatch follows the API's PATCH rules: an omitted key leaves the field unchanged; neither field is nullable.
export type BoardPatch = {
  name?: string
  taskPrefix?: string
}

export type LabelPatch = {
  name?: string
  color?: LabelColor
}

export const GITHUB_CONNECTION_STATES = ['disconnected', 'connected', 'unreadable'] as const
export type GithubConnectionState = (typeof GITHUB_CONNECTION_STATES)[number]

// GithubConnection carries the App fields only while an App is stored; webhookUrl is always there so screens can warn early.
export type GithubConnection = {
  state: GithubConnectionState
  webhookUrl: string
  appSlug?: string
  ownerLogin?: string
  appUrl?: string
  installUrl?: string
}

// GithubManifestStart.manifest is opaque to the web app: it is posted to GitHub exactly as the API built it.
export type GithubManifestStart = {
  formAction: string
  manifest: Record<string, unknown>
  state: string
}

// BoardGithubSettings uses null for "do not move" on an event.
export type BoardGithubSettings = {
  openedColumnToken: string | null
  reviewRequestedColumnToken: string | null
  mergedColumnToken: string | null
  finalBranches: string[]
}

// BoardGithubSettingsPatch follows the API's PATCH rules: an omitted key is unchanged, null clears a column; finalBranches is not nullable.
export type BoardGithubSettingsPatch = Partial<BoardGithubSettings>
