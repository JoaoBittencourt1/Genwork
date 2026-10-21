export const GENTASKS_PERMISSIONS = {
  boardRead: 'gentasks.board.read',
  boardManage: 'gentasks.board.manage',
  taskWrite: 'gentasks.task.write',
  taskDelete: 'gentasks.task.delete',
} as const

export type GentasksAbilities = {
  canRead: boolean
  canManageBoards: boolean
  canWriteTasks: boolean
  canDeleteTasks: boolean
}

export function gentasksAbilities(can: (permission: string) => boolean): GentasksAbilities {
  return {
    canRead: can(GENTASKS_PERMISSIONS.boardRead),
    canManageBoards: can(GENTASKS_PERMISSIONS.boardManage),
    canWriteTasks: can(GENTASKS_PERMISSIONS.taskWrite),
    canDeleteTasks: can(GENTASKS_PERMISSIONS.taskDelete),
  }
}
