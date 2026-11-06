import { createContext, useContext } from 'react'
import type { BotOption } from '../domain/botAssignees'

export type BotAssignment = {
  // genagentsEnabled hides every bot feature of the board while the module is off (rule 58).
  genagentsEnabled: boolean
  canReadRuns: boolean
  canWriteRuns: boolean
  botOptions: BotOption[] | undefined
  reviewerBots: { token: string; name: string; memberToken: string }[]
}

export const NO_BOT_ASSIGNMENT: BotAssignment = { genagentsEnabled: false, canReadRuns: false, canWriteRuns: false, botOptions: undefined, reviewerBots: [] }

export const BotAssignmentContext = createContext<BotAssignment>(NO_BOT_ASSIGNMENT)

export function useBotAssignment(): BotAssignment {
  return useContext(BotAssignmentContext)
}
