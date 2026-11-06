import { apiRequest } from '@/shared/datasource/httpClient'
import type { ConversationPage } from '@/shared/domain/conversation'
import type { BotOption } from '../domain/botAssignees'

// gentasks reaches genagents only through its public API (rule 54), from its own datasource (rule 59).
const GENAGENTS_API = '/api/genagents/v1'

type ListedBot = { token: string; name: string; kind: BotOption['kind']; memberToken: string; enabled: boolean }

export type ReviewerBot = BotOption & { token: string }

export async function listBotOptions(): Promise<ReviewerBot[]> {
  const bots = (await apiRequest<{ bots: ListedBot[] }>('GET', `${GENAGENTS_API}/bots`)).bots
  return bots.map((bot) => ({ token: bot.token, memberToken: bot.memberToken, name: bot.name, kind: bot.kind, enabled: bot.enabled }))
}

function conversationPath(taskToken: string): string {
  return `/api/gentasks/v1/tasks/${encodeURIComponent(taskToken)}`
}

// The conversation of a task and the bot review go through gentasks, which relays them to genagents.
export function readTaskConversation(taskToken: string, afterSequence: number): Promise<ConversationPage> {
  return apiRequest<ConversationPage>('GET', `${GENAGENTS_API}/tasks/${encodeURIComponent(taskToken)}/conversation?after=${afterSequence}`)
}

export function postTaskMessage(taskToken: string, text: string): Promise<void> {
  return apiRequest<void>('POST', `${GENAGENTS_API}/tasks/${encodeURIComponent(taskToken)}/conversation/messages`, { body: { text } })
}

export function stopTaskRun(taskToken: string): Promise<void> {
  return apiRequest<void>('POST', `${GENAGENTS_API}/tasks/${encodeURIComponent(taskToken)}/conversation/stop`)
}

export function requestBotReview(taskToken: string, botToken?: string): Promise<{ runToken: string; state: string }> {
  return apiRequest<{ runToken: string; state: string }>('POST', `${conversationPath(taskToken)}/bot-reviews`, { body: botToken ? { botToken } : {} })
}
