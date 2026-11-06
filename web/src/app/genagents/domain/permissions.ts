export const GENAGENTS_PERMISSIONS = {
  accountManage: 'genagents.account.manage',
  botRead: 'genagents.bot.read',
  botManage: 'genagents.bot.manage',
  skillManage: 'genagents.skill.manage',
  mcpServerManage: 'genagents.mcpserver.manage',
  runnerManage: 'genagents.runner.manage',
  runRead: 'genagents.run.read',
  runWrite: 'genagents.run.write',
  adminChatUse: 'genagents.adminchat.use',
} as const

export type GenagentsAbilities = {
  canManageAccounts: boolean
  canReadBots: boolean
  canManageBots: boolean
  canManageSkills: boolean
  canManageMcpServers: boolean
  canManageRunners: boolean
  canReadRuns: boolean
  canWriteRuns: boolean
  canUseAdminChat: boolean
}

export function genagentsAbilities(can: (permission: string) => boolean): GenagentsAbilities {
  return {
    canManageAccounts: can(GENAGENTS_PERMISSIONS.accountManage),
    canReadBots: can(GENAGENTS_PERMISSIONS.botRead),
    canManageBots: can(GENAGENTS_PERMISSIONS.botManage),
    canManageSkills: can(GENAGENTS_PERMISSIONS.skillManage),
    canManageMcpServers: can(GENAGENTS_PERMISSIONS.mcpServerManage),
    canManageRunners: can(GENAGENTS_PERMISSIONS.runnerManage),
    canReadRuns: can(GENAGENTS_PERMISSIONS.runRead),
    canWriteRuns: can(GENAGENTS_PERMISSIONS.runWrite),
    canUseAdminChat: can(GENAGENTS_PERMISSIONS.adminChatUse),
  }
}

// canSeeAiSection shows the AI navigation section to anyone who may open at least one of its pages.
export function canSeeAiSection(abilities: GenagentsAbilities): boolean {
  return Object.values(abilities).some(Boolean)
}
