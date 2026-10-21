export const gentasksQueryKeys = {
  boards: ['gentasks', 'boards'] as const,
  board: (boardToken: string) => ['gentasks', 'board', boardToken] as const,
  archived: (boardToken: string) => ['gentasks', 'archived', boardToken] as const,
  assignedTasks: (limit: number) => ['gentasks', 'assigned-tasks', limit] as const,
}
