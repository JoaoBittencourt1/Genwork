import type { Bot, BotPatch } from './types'

type BotDraft = {
  name: string
  kind: Bot['kind']
  accountToken: string
  model: string
  instructions: string
  effort: Bot['effort']
  maxTurns: number
  timeoutMinutes: number
  skillTokens: string[]
  mcpServerTokens: string[]
  enabled: boolean
}

export function draftFromBot(bot: Bot): BotDraft {
  return {
    name: bot.name,
    kind: bot.kind,
    accountToken: bot.account.token,
    model: bot.model,
    instructions: bot.instructions ?? '',
    effort: bot.effort,
    maxTurns: bot.maxTurns,
    timeoutMinutes: bot.timeoutMinutes,
    skillTokens: bot.skills.map((skill) => skill.token),
    mcpServerTokens: bot.mcpServers.map((server) => server.token),
    enabled: bot.enabled,
  }
}

function sameTokens(left: string[], right: string[]): boolean {
  return left.length === right.length && [...left].sort().every((token, index) => token === [...right].sort()[index])
}

// buildBotPatch sends only what changed, so saving one field never rewrites the others (rule 17); cleared instructions
// are sent as null.
export function buildBotPatch(bot: Bot, draft: BotDraft): BotPatch {
  const original = draftFromBot(bot)
  const patch: BotPatch = {}
  if (draft.name.trim() !== original.name) patch.name = draft.name.trim()
  if (draft.kind !== original.kind) patch.kind = draft.kind
  if (draft.accountToken !== original.accountToken) patch.accountToken = draft.accountToken
  if (draft.model !== original.model) patch.model = draft.model
  if (draft.effort !== original.effort) patch.effort = draft.effort
  if (draft.maxTurns !== original.maxTurns) patch.maxTurns = draft.maxTurns
  if (draft.timeoutMinutes !== original.timeoutMinutes) patch.timeoutMinutes = draft.timeoutMinutes
  if (draft.enabled !== original.enabled) patch.enabled = draft.enabled
  if (draft.instructions !== original.instructions) patch.instructions = draft.instructions.trim() === '' ? null : draft.instructions
  if (!sameTokens(draft.skillTokens, original.skillTokens)) patch.skillTokens = draft.skillTokens
  if (!sameTokens(draft.mcpServerTokens, original.mcpServerTokens)) patch.mcpServerTokens = draft.mcpServerTokens
  return patch
}

export function isEmptyPatch(patch: object): boolean {
  return Object.keys(patch).length === 0
}
