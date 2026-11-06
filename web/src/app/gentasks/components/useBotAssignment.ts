import { useQuery } from '@tanstack/react-query'
import { useAuth } from '@/app/auth/components/AuthContext'
import { useCapabilities } from '@/shared/components/CapabilitiesContext'
import { isModuleEnabled, MODULE_IDS } from '@/shared/domain/capabilities'
import { listBotOptions } from '../datasource/botConversationDatasource'
import { GENAGENTS_PERMISSIONS_FOR_GENTASKS } from '../domain/permissions'
import type { BotAssignment } from './BotAssignmentContext'

// useBotAssignment reads genagents only while it is enabled and the caller may read bots; otherwise the board has no bots.
export function useBotAssignmentValue(): BotAssignment {
  const { can } = useAuth()
  const { enabledModules } = useCapabilities()
  const genagentsEnabled = isModuleEnabled(enabledModules, MODULE_IDS.genagents)
  const canReadBots = genagentsEnabled && can(GENAGENTS_PERMISSIONS_FOR_GENTASKS.botRead)
  const bots = useQuery({ queryKey: ['gentasks', 'bot-options'], queryFn: listBotOptions, enabled: canReadBots })
  const options = bots.data
  return {
    genagentsEnabled,
    canReadRuns: genagentsEnabled && can(GENAGENTS_PERMISSIONS_FOR_GENTASKS.runRead),
    canWriteRuns: genagentsEnabled && can(GENAGENTS_PERMISSIONS_FOR_GENTASKS.runWrite),
    botOptions: options,
    reviewerBots: (options ?? []).filter((bot) => bot.kind === 'reviewer' && bot.enabled).map((bot) => ({ token: bot.token, name: bot.name, memberToken: bot.memberToken })),
  }
}
