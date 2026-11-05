export const GENTASKS_PERMISSIONS = {
  boardRead: 'gentasks.board.read',
  boardManage: 'gentasks.board.manage',
  taskWrite: 'gentasks.task.write',
  taskDelete: 'gentasks.task.delete',
  githubManage: 'gentasks.github.manage',
} as const

export type GentasksAbilities = {
  canRead: boolean
  canManageBoards: boolean
  canWriteTasks: boolean
  canDeleteTasks: boolean
  canManageGithub: boolean
}

export function gentasksAbilities(can: (permission: string) => boolean): GentasksAbilities {
  return {
    canRead: can(GENTASKS_PERMISSIONS.boardRead),
    canManageBoards: can(GENTASKS_PERMISSIONS.boardManage),
    canWriteTasks: can(GENTASKS_PERMISSIONS.taskWrite),
    canDeleteTasks: can(GENTASKS_PERMISSIONS.taskDelete),
    canManageGithub: can(GENTASKS_PERMISSIONS.githubManage),
  }
}

// GENAGENTS_PERMISSIONS_FOR_GENTASKS names the genagents permissions the task dialog checks to show its bot features.
export const GENAGENTS_PERMISSIONS_FOR_GENTASKS = {
  botRead: 'genagents.bot.read',
  runRead: 'genagents.run.read',
  runWrite: 'genagents.run.write',
} as const
