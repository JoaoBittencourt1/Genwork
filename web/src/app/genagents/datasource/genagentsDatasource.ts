import { apiRequest } from '@/shared/datasource/httpClient'
import type { ConversationPage } from '@/shared/domain/conversation'
import type {
  Account,
  AccountPatch,
  AccountSignIn,
  Bot,
  BotKind,
  BotPatch,
  Chat,
  ChatSummary,
  CreatedRunner,
  McpServer,
  McpServerPatch,
  NewAccount,
  NewBot,
  NewMcpServer,
  NewSkill,
  ProviderCatalogEntry,
  Runner,
  Skill,
  SkillPatch,
  StartMcpOAuth,
} from '../domain/types'

const API = '/api/genagents/v1'

function segment(token: string): string {
  return encodeURIComponent(token)
}

export async function listProviders(): Promise<ProviderCatalogEntry[]> {
  return (await apiRequest<{ providers: ProviderCatalogEntry[] }>('GET', `${API}/providers`)).providers
}

export async function listAccounts(): Promise<Account[]> {
  return (await apiRequest<{ accounts: Account[] }>('GET', `${API}/accounts`)).accounts
}

export function createAccount(account: NewAccount): Promise<Account> {
  return apiRequest<Account>('POST', `${API}/accounts`, { body: account })
}

export function startAccountSignIn(accountToken: string): Promise<AccountSignIn> {
  return apiRequest<AccountSignIn>('POST', `${API}/accounts/${segment(accountToken)}/sign-ins`)
}

export function submitAccountSignInCode(accountToken: string, signInToken: string, code: string): Promise<AccountSignIn> {
  return apiRequest<AccountSignIn>('POST', `${API}/accounts/${segment(accountToken)}/sign-ins/${segment(signInToken)}/code`, { body: { code } })
}

export function cancelAccountSignIn(accountToken: string, signInToken: string): Promise<void> {
  return apiRequest<void>('DELETE', `${API}/accounts/${segment(accountToken)}/sign-ins/${segment(signInToken)}`)
}

export function updateAccount(accountToken: string, patch: AccountPatch): Promise<Account> {
  return apiRequest<Account>('PATCH', `${API}/accounts/${segment(accountToken)}`, { body: patch })
}

export function removeAccount(accountToken: string): Promise<void> {
  return apiRequest<void>('DELETE', `${API}/accounts/${segment(accountToken)}`)
}

export function addCustomModel(accountToken: string, modelId: string, displayName?: string): Promise<Account> {
  return apiRequest<Account>('POST', `${API}/accounts/${segment(accountToken)}/models`, { body: { modelId, displayName: displayName ?? '' } })
}

export function removeCustomModel(accountToken: string, modelToken: string): Promise<void> {
  return apiRequest<void>('DELETE', `${API}/accounts/${segment(accountToken)}/models/${segment(modelToken)}`)
}

export async function listBots(kind?: BotKind): Promise<Bot[]> {
  const query = kind ? `?kind=${kind}` : ''
  return (await apiRequest<{ bots: Bot[] }>('GET', `${API}/bots${query}`)).bots
}

export function createBot(bot: NewBot): Promise<Bot> {
  return apiRequest<Bot>('POST', `${API}/bots`, { body: bot })
}

export function updateBot(botToken: string, patch: BotPatch): Promise<Bot> {
  return apiRequest<Bot>('PATCH', `${API}/bots/${segment(botToken)}`, { body: patch })
}

export function deleteBot(botToken: string): Promise<void> {
  return apiRequest<void>('DELETE', `${API}/bots/${segment(botToken)}`)
}

export function rotateBotSecret(botToken: string): Promise<void> {
  return apiRequest<void>('POST', `${API}/bots/${segment(botToken)}/secret`)
}

export async function listSkills(): Promise<Skill[]> {
  return (await apiRequest<{ skills: Skill[] }>('GET', `${API}/skills`)).skills
}

export function createSkill(skill: NewSkill): Promise<Skill> {
  return apiRequest<Skill>('POST', `${API}/skills`, { body: skill })
}

export function importSkill(content: string): Promise<Skill> {
  return apiRequest<Skill>('POST', `${API}/skills/import`, { body: { content } })
}

export function updateSkill(skillToken: string, patch: SkillPatch): Promise<Skill> {
  return apiRequest<Skill>('PATCH', `${API}/skills/${segment(skillToken)}`, { body: patch })
}

export function deleteSkill(skillToken: string): Promise<void> {
  return apiRequest<void>('DELETE', `${API}/skills/${segment(skillToken)}`)
}

export async function listMcpServers(): Promise<McpServer[]> {
  return (await apiRequest<{ servers: McpServer[] }>('GET', `${API}/mcp-servers`)).servers
}

export function createMcpServer(server: NewMcpServer): Promise<McpServer> {
  return apiRequest<McpServer>('POST', `${API}/mcp-servers`, { body: server })
}

export function updateMcpServer(serverToken: string, patch: McpServerPatch): Promise<McpServer> {
  return apiRequest<McpServer>('PATCH', `${API}/mcp-servers/${segment(serverToken)}`, { body: patch })
}

// startMcpOAuth returns the authorization server's page the browser opens to connect the server.
export async function startMcpOAuth(serverToken: string, input: StartMcpOAuth = {}): Promise<string> {
  return (await apiRequest<{ authorizationUrl: string }>('POST', `${API}/mcp-servers/${segment(serverToken)}/oauth/authorizations`, { body: input })).authorizationUrl
}

export function disconnectMcpOAuth(serverToken: string): Promise<void> {
  return apiRequest<void>('DELETE', `${API}/mcp-servers/${segment(serverToken)}/oauth`)
}

export function deleteMcpServer(serverToken: string): Promise<void> {
  return apiRequest<void>('DELETE', `${API}/mcp-servers/${segment(serverToken)}`)
}

export async function listRunners(): Promise<Runner[]> {
  return (await apiRequest<{ runners: Runner[] }>('GET', `${API}/runners`)).runners
}

export function createRunner(name: string): Promise<CreatedRunner> {
  return apiRequest<CreatedRunner>('POST', `${API}/runners`, { body: { name } })
}

export function revokeRunner(runnerToken: string): Promise<void> {
  return apiRequest<void>('DELETE', `${API}/runners/${segment(runnerToken)}`)
}

export function removeRunner(runnerToken: string): Promise<void> {
  return apiRequest<void>('POST', `${API}/runners/${segment(runnerToken)}/removal`)
}

export async function listChats(): Promise<ChatSummary[]> {
  return (await apiRequest<{ chats: ChatSummary[] }>('GET', `${API}/admin-chats`)).chats
}

export function startChat(botToken: string, title?: string): Promise<ChatSummary> {
  return apiRequest<ChatSummary>('POST', `${API}/admin-chats`, { body: { botToken, title: title ?? '' } })
}

// readChat reads only the entries after a sequence, so polling stays cheap.
export function readChat(chatToken: string, afterSequence: number): Promise<Chat> {
  return apiRequest<Chat>('GET', `${API}/admin-chats/${segment(chatToken)}?after=${afterSequence}`)
}

export function renameChat(chatToken: string, title: string): Promise<void> {
  return apiRequest<void>('PATCH', `${API}/admin-chats/${segment(chatToken)}`, { body: { title } })
}

export function deleteChat(chatToken: string): Promise<void> {
  return apiRequest<void>('DELETE', `${API}/admin-chats/${segment(chatToken)}`)
}

export function sendChatMessage(chatToken: string, text: string): Promise<void> {
  return apiRequest<void>('POST', `${API}/admin-chats/${segment(chatToken)}/messages`, { body: { text } })
}

export function stopChat(chatToken: string): Promise<void> {
  return apiRequest<void>('POST', `${API}/admin-chats/${segment(chatToken)}/stop`)
}

export function reopenChat(chatToken: string): Promise<void> {
  return apiRequest<void>('POST', `${API}/admin-chats/${segment(chatToken)}/reopen`)
}

export type ChatPage = ConversationPage & { botName: string }
