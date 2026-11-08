import type { ConversationPage } from '@/shared/domain/conversation'

export const PROVIDERS = ['anthropic', 'openai', 'cursor', 'xai'] as const
export type Provider = (typeof PROVIDERS)[number]

export const ACCOUNT_MODES = ['subscription', 'api_key'] as const
export type AccountMode = (typeof ACCOUNT_MODES)[number]

export const ACCOUNT_STATUSES = ['pending_sign_in', 'ready', 'error'] as const
export type AccountStatus = (typeof ACCOUNT_STATUSES)[number]

export const SIGN_IN_STATES = ['requested', 'awaiting_user', 'code_submitted', 'verifying', 'succeeded', 'failed', 'cancelled', 'expired'] as const
export type SignInState = (typeof SIGN_IN_STATES)[number]

// AccountSignIn is one web-started sign-in attempt; the code a person pastes is never part of it.
export type AccountSignIn = {
  token: string
  state: SignInState
  url: string | null
  urlHost: string | null
  userCode: string | null
  expectsCode: boolean
  failure: string | null
  expiresAt: string
  createdAt: string
}

export const BOT_KINDS = ['dev', 'reviewer', 'assistant'] as const
export type BotKind = (typeof BOT_KINDS)[number]

export const REASONING_EFFORTS = ['default', 'low', 'medium', 'high'] as const
export type ReasoningEffort = (typeof REASONING_EFFORTS)[number]

export const MCP_TRANSPORTS = ['http', 'stdio'] as const
export type McpTransport = (typeof MCP_TRANSPORTS)[number]

export type ProviderModel = { token: string | null; id: string; displayName: string; custom: boolean }

export type ProviderCatalogEntry = { provider: Provider; cli: string; modes: AccountMode[]; models: ProviderModel[] }

export type Account = {
  token: string
  name: string
  provider: Provider
  mode: AccountMode
  runner: { token: string; name: string } | null
  hasApiKey: boolean
  apiKeyLast4: string | null
  status: AccountStatus
  lastError: string | null
  signInCommand: string | null
  signIn: AccountSignIn | null
  models: ProviderModel[]
  createdAt: string
}

export type NewAccount = {
  name: string
  provider: Provider
  mode: AccountMode
  apiKey?: string
  runnerToken?: string
}

export type AccountPatch = { name?: string; apiKey?: string }

export type Reference = { token: string; name: string }

export type Bot = {
  token: string
  name: string
  kind: BotKind
  provider: Provider
  model: string
  memberToken: string
  enabled: boolean
  account: Reference
  effort: ReasoningEffort
  maxTurns: number
  timeoutMinutes: number
  instructions: string | null
  skills: Reference[]
  mcpServers: Reference[]
  updatedAt: string
}

export type NewBot = {
  name: string
  kind: BotKind
  accountToken: string
  model: string
  role: string
  instructions?: string
  effort: ReasoningEffort
  maxTurns: number
  timeoutMinutes: number
  skillTokens: string[]
  mcpServerTokens: string[]
}

// BotPatch follows the API's PATCH rules: an omitted key is unchanged, and only instructions accepts null.
export type BotPatch = Partial<Omit<NewBot, 'instructions'>> & { instructions?: string | null; enabled?: boolean }

export type SkillFile = { path: string; content: string }

export type Skill = { token: string; name: string; description: string; body: string; version: number; files: SkillFile[]; updatedAt: string }

export type NewSkill = { name: string; description: string; body: string; files: SkillFile[] }

export type SkillPatch = Partial<NewSkill>

export type McpEntry = { name: string; secret: boolean; hasValue: boolean; value: string | null }

export const MCP_OAUTH_STATUSES = ['connected', 'error'] as const
export type McpOAuthStatus = (typeof MCP_OAUTH_STATUSES)[number]

// McpOAuthConnection never carries a token or the client secret.
export type McpOAuthConnection = {
  status: McpOAuthStatus
  clientId: string | null
  connectedAt: string | null
  expiresAt: string | null
  lastError: string | null
}

export type McpServer = {
  token: string
  name: string
  transport: McpTransport
  url: string | null
  headers: McpEntry[]
  command: string | null
  args: string[]
  env: McpEntry[]
  oauth: McpOAuthConnection | null
  updatedAt: string
}

export type StartMcpOAuth = { clientId?: string; clientSecret?: string; scope?: string }

export type McpEntryInput = { name: string; secret: boolean; value?: string }

export type NewMcpServer = {
  name: string
  transport: McpTransport
  url?: string
  headers?: McpEntryInput[]
  command?: string
  args?: string[]
  env?: McpEntryInput[]
}

export type McpServerPatch = Partial<Omit<NewMcpServer, 'transport'>>

export type Runner = {
  token: string
  name: string
  online: boolean
  revoked: boolean
  registered: boolean
  lastHeartbeatAt: string | null
  registrationExpiresAt: string | null
  containerEngine: string | null
  sandboxImage: string | null
  customImage: boolean
  directoryMode: boolean
  hasGithubCredentials: boolean
  maxConcurrency: number
  tools: Record<string, string>
  cleanupPending: boolean
  lastCleanupAt: string | null
  lastCleanupFreedBytes: number | null
}

// RunnersCleanup is what freeing space removed in genagents and how many runners were asked to clean up.
export type RunnersCleanup = { removedSignIns: number; removedAuthorizations: number; runnersAsked: number }

export type CreatedRunner = Runner & { registrationToken: string; registrationExpiresAt: string }

export type ChatSummary = { token: string; title: string; updatedAt: string }

export type Chat = ChatSummary & ConversationPage & { botName: string }
