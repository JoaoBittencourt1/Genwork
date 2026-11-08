import type { Account, AccountSignIn, Bot, McpServer, Runner, Skill } from '@/app/genagents/domain/types'

const catalogModels = [
  { token: null, id: 'claude-sonnet-5-5', displayName: 'Claude Sonnet 5.5', custom: false },
  { token: null, id: 'claude-opus-5-5', displayName: 'Claude Opus 5.5', custom: false },
]

export function accountFixture(overrides: Partial<Account> = {}): Account {
  return {
    token: 'account-1',
    name: 'Claude — Team',
    provider: 'anthropic',
    mode: 'api_key',
    runner: null,
    hasApiKey: true,
    apiKeyLast4: '0001',
    status: 'ready',
    lastError: null,
    signInCommand: null,
    signIn: null,
    models: catalogModels,
    createdAt: '2026-11-05T12:00:00Z',
    ...overrides,
  }
}

export function botFixture(overrides: Partial<Bot> = {}): Bot {
  return {
    token: 'bot-1',
    name: 'Dev Bot',
    kind: 'dev',
    provider: 'anthropic',
    model: 'claude-sonnet-5-5',
    memberToken: 'member-bot-1',
    enabled: true,
    account: { token: 'account-1', name: 'Claude — Team' },
    effort: 'default',
    maxTurns: 100,
    timeoutMinutes: 60,
    instructions: null,
    skills: [],
    mcpServers: [],
    updatedAt: '2026-11-05T12:00:00Z',
    ...overrides,
  }
}

export function skillFixture(overrides: Partial<Skill> = {}): Skill {
  return { token: 'skill-1', name: 'go-conventions', description: 'How we write Go', body: '# Go', version: 1, files: [], updatedAt: '2026-11-05T12:00:00Z', ...overrides }
}

export function mcpServerFixture(overrides: Partial<McpServer> = {}): McpServer {
  return {
    token: 'server-1',
    name: 'sentry',
    transport: 'http',
    url: 'https://mcp.sentry.dev/mcp',
    headers: [{ name: 'Authorization', secret: true, hasValue: true, value: null }],
    command: null,
    args: [],
    env: [],
    oauth: null,
    updatedAt: '2026-11-05T12:00:00Z',
    ...overrides,
  }
}

export function signInFixture(overrides: Partial<AccountSignIn> = {}): AccountSignIn {
  return {
    token: 'sign-in-1',
    state: 'requested',
    url: null,
    urlHost: null,
    userCode: null,
    expectsCode: false,
    failure: null,
    expiresAt: '2026-11-05T12:10:00Z',
    createdAt: '2026-11-05T12:00:00Z',
    ...overrides,
  }
}

// subscriptionAccountFixture is a Claude subscription account held by a runner, as a web sign-in needs.
export function subscriptionAccountFixture(overrides: Partial<Account> = {}): Account {
  return accountFixture({
    mode: 'subscription',
    runner: { token: 'runner-1', name: 'build-box' },
    hasApiKey: false,
    apiKeyLast4: null,
    status: 'pending_sign_in',
    signInCommand: 'genwork-runner account login account-1',
    ...overrides,
  })
}

export function runnerFixture(overrides: Partial<Runner> = {}): Runner {
  return {
    token: 'runner-1',
    name: 'build-box',
    online: true,
    revoked: false,
    registered: true,
    lastHeartbeatAt: '2026-11-05T12:00:00Z',
    registrationExpiresAt: null,
    containerEngine: 'Docker 27',
    sandboxImage: 'ghcr.io/acme/genwork-sandbox:2.0.0',
    customImage: false,
    directoryMode: false,
    hasGithubCredentials: true,
    maxConcurrency: 2,
    tools: { claude: '2.1.0' },
    ...overrides,
  }
}
