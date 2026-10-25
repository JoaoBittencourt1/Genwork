import type {
  AssignedTask,
  BoardGithubSettings,
  BoardTask,
  BoardView,
  ColumnWithTasks,
  GithubConnection,
  Label,
  Task,
  TaskPullRequest,
} from '@/app/gentasks/domain/types'
import type { Capabilities } from '@/shared/datasource/capabilities'
import type { DirectoryUser } from '@/shared/datasource/users'

export const capabilitiesFixture: Capabilities = {
  core: { version: '1.0.0', api: '1.0.0' },
  modules: [{ id: 'gentasks', version: '0.3.0', apiPrefix: '/api/gentasks/v1' }],
}

export const usersFixture: DirectoryUser[] = [
  { token: 'user-ana', name: 'Ana Souza', email: 'ana@example.com', avatarColor: 'blue' },
  { token: 'user-bruno', name: 'Bruno', email: 'bruno@example.com', avatarColor: 'pink' },
]

export const bugLabel: Label = { token: 'label-bug', name: 'Bug', color: 'red' }
export const featureLabel: Label = { token: 'label-feature', name: 'Feature', color: 'green' }

export function taskFixture(overrides: Partial<Task> = {}): Task {
  return {
    token: 'task-1',
    number: 1,
    columnToken: 'column-backlog',
    position: 0,
    title: 'Write tests',
    description: null,
    assigneeToken: null,
    urgency: 'none',
    labels: [],
    createdBy: 'user-ana',
    assignedAt: null,
    archivedAt: null,
    createdAt: '2026-10-21T12:00:00Z',
    updatedAt: '2026-10-21T12:00:00Z',
    ...overrides,
  }
}

export function assignedTaskFixture(overrides: Partial<AssignedTask> = {}): AssignedTask {
  return {
    ...taskFixture({ assigneeToken: 'user-ana', assignedAt: '2026-10-18T12:00:00Z' }),
    board: { token: 'board-1', name: 'Platform', taskPrefix: 'PLA' },
    column: { token: 'column-backlog', preset: 'backlog', name: null },
    ...overrides,
  }
}

type ColumnFixtureTasks = { titles: string[]; firstNumber: number }

export function pullRequestFixture(overrides: Partial<TaskPullRequest> = {}): TaskPullRequest {
  const number = overrides.number ?? 12
  return {
    token: `pull-request-${number}`,
    repository: 'acme/api',
    number,
    title: 'PLA-1: sign in with e-mail',
    state: 'open',
    author: { login: 'ana-dev', avatarUrl: 'https://avatars.githubusercontent.com/u/5001?v=4' },
    url: `https://github.com/acme/api/pull/${number}`,
    ...overrides,
  }
}

export function boardTaskFixture(overrides: Partial<BoardTask> = {}): BoardTask {
  return { ...taskFixture(overrides), pullRequests: [], ...overrides }
}

function columnFixture(token: string, position: number, { titles, firstNumber }: ColumnFixtureTasks, overrides: Partial<ColumnWithTasks> = {}): ColumnWithTasks {
  return {
    token,
    preset: null,
    name: null,
    position,
    tasks: titles.map((title, index) =>
      boardTaskFixture({
        token: `task-${title}`,
        number: firstNumber + index,
        title,
        columnToken: token,
        position: index,
        pullRequests: firstNumber + index === 1 ? [pullRequestFixture()] : [],
      }),
    ),
    ...overrides,
  }
}

// boardViewFixture: backlog [A, B, C], todo [D], done [] — the default columns with tasks PLA-1 to PLA-4; PLA-1 has a
// pull request.
export function boardViewFixture(): BoardView {
  return {
    token: 'board-1',
    name: 'Platform',
    taskPrefix: 'PLA',
    createdBy: 'user-ana',
    createdAt: '2026-10-21T12:00:00Z',
    updatedAt: '2026-10-21T12:00:00Z',
    columns: [
      columnFixture('column-backlog', 0, { titles: ['A', 'B', 'C'], firstNumber: 1 }, { preset: 'backlog' }),
      columnFixture('column-todo', 1, { titles: ['D'], firstNumber: 4 }, { preset: 'todo' }),
      columnFixture('column-done', 2, { titles: [], firstNumber: 5 }, { preset: 'done' }),
    ],
    labels: [bugLabel, featureLabel],
  }
}

export function titlesByColumn(view: BoardView): Record<string, string[]> {
  return Object.fromEntries(view.columns.map((column) => [column.token, column.tasks.map((task) => task.title)]))
}

export const githubWebhookUrlFixture = 'https://genwork.acme.com/api/gentasks/v1/github/webhook'

export function githubConnectionFixture(overrides: Partial<GithubConnection> = {}): GithubConnection {
  return {
    state: 'connected',
    webhookUrl: githubWebhookUrlFixture,
    appSlug: 'genwork-k3f9qz',
    ownerLogin: 'acme',
    appUrl: 'https://github.com/organizations/acme/settings/apps/genwork-k3f9qz',
    installUrl: 'https://github.com/apps/genwork-k3f9qz/installations/new',
    ...overrides,
  }
}

export const disconnectedGithubFixture: GithubConnection = { state: 'disconnected', webhookUrl: githubWebhookUrlFixture }

export function boardGithubSettingsFixture(overrides: Partial<BoardGithubSettings> = {}): BoardGithubSettings {
  return {
    openedColumnToken: null,
    reviewRequestedColumnToken: null,
    mergedColumnToken: 'column-done',
    finalBranches: ['main', 'master'],
    ...overrides,
  }
}
