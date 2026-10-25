export const gentasksQueryKeys = {
  boards: ['gentasks', 'boards'] as const,
  board: (boardToken: string) => ['gentasks', 'board', boardToken] as const,
  archived: (boardToken: string) => ['gentasks', 'archived', boardToken] as const,
  assignedTasks: (limit: number) => ['gentasks', 'assigned-tasks', limit] as const,
  githubConnection: ['gentasks', 'github-connection'] as const,
  boardGithubSettings: (boardToken: string) => ['gentasks', 'board-github-settings', boardToken] as const,
}

export const gentasksMutationKeys = {
  boardMoves: (boardToken: string) => ['gentasks', 'board-moves', boardToken] as const,
}
