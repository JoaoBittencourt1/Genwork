import { HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { setAccessToken } from '@/shared/datasource/httpClient'
import { accountFixture, botFixture, mcpServerFixture, runnerFixture, signInFixture, skillFixture } from '@/test/genagentsFixtures'
import { captureRequests, errorResponse } from '@/test/server'
import {
  addCustomModel,
  cancelAccountSignIn,
  createAccount,
  createBot,
  createMcpServer,
  createRunner,
  createSkill,
  deleteBot,
  deleteChat,
  deleteMcpServer,
  deleteSkill,
  disconnectMcpOAuth,
  importSkill,
  listAccounts,
  listBots,
  listChats,
  listMcpServers,
  listProviders,
  listRunners,
  listSkills,
  readChat,
  removeAccount,
  removeCustomModel,
  reopenChat,
  renameChat,
  removeRunner,
  revokeRunner,
  rotateBotSecret,
  sendChatMessage,
  startAccountSignIn,
  startChat,
  startMcpOAuth,
  stopChat,
  submitAccountSignInCode,
  updateAccount,
  updateBot,
  updateMcpServer,
  updateSkill,
} from './genagentsDatasource'

const noContent = () => new HttpResponse(null, { status: 204 })

describe('accounts', () => {
  it('lists providers and accounts, and manages an account and its custom models', async () => {
    const account = accountFixture()
    captureRequests('get', '*/api/genagents/v1/providers', () => HttpResponse.json({ providers: [{ provider: 'xai', cli: 'grok', modes: ['api_key'], models: [] }] }))
    captureRequests('get', '*/api/genagents/v1/accounts', () => HttpResponse.json({ accounts: [account] }))
    const created = captureRequests('post', '*/api/genagents/v1/accounts', () => HttpResponse.json(account, { status: 201 }))
    const patched = captureRequests('patch', '*/api/genagents/v1/accounts/:token', () => HttpResponse.json(account))
    const removed = captureRequests('delete', '*/api/genagents/v1/accounts/:token', noContent)
    const modelAdded = captureRequests('post', '*/api/genagents/v1/accounts/:token/models', () => HttpResponse.json(account, { status: 201 }))
    const modelRemoved = captureRequests('delete', '*/api/genagents/v1/accounts/:token/models/:model', noContent)
    setAccessToken('token-1')

    await expect(listProviders()).resolves.toEqual([{ provider: 'xai', cli: 'grok', modes: ['api_key'], models: [] }])
    await expect(listAccounts()).resolves.toEqual([account])
    await createAccount({ name: 'Claude', provider: 'anthropic', mode: 'api_key', apiKey: 'sk-1' })
    await updateAccount('account/1', { name: 'Renamed' })
    await removeAccount('account-1')
    await addCustomModel('account-1', 'claude-next', 'Next')
    await removeCustomModel('account-1', 'model-1')

    expect(created[0]?.body).toEqual({ name: 'Claude', provider: 'anthropic', mode: 'api_key', apiKey: 'sk-1' })
    expect(created[0]?.headers.get('Authorization')).toBe('Bearer token-1')
    expect(patched[0]?.path).toBe('/api/genagents/v1/accounts/account%2F1')
    expect(patched[0]?.body).toEqual({ name: 'Renamed' })
    expect(removed[0]?.path).toBe('/api/genagents/v1/accounts/account-1')
    expect(modelAdded[0]?.body).toEqual({ modelId: 'claude-next', displayName: 'Next' })
    expect(modelRemoved[0]?.path).toBe('/api/genagents/v1/accounts/account-1/models/model-1')
  })

  it('maps an account in use to its error with the bot names', async () => {
    captureRequests('delete', '*/api/genagents/v1/accounts/:token', () => errorResponse(409, 'account_in_use', { bots: 'Dev Bot' }))
    setAccessToken('token-1')

    await expect(removeAccount('account-1')).rejects.toMatchObject({ status: 409, code: 'account_in_use', fields: { bots: 'Dev Bot' } })
  })
})

describe('bots', () => {
  it('lists bots, optionally of one kind, and manages one', async () => {
    const bot = botFixture()
    const listed = captureRequests('get', '*/api/genagents/v1/bots', () => HttpResponse.json({ bots: [bot] }))
    const created = captureRequests('post', '*/api/genagents/v1/bots', () => HttpResponse.json(bot, { status: 201 }))
    const patched = captureRequests('patch', '*/api/genagents/v1/bots/:token', () => HttpResponse.json(bot))
    const deleted = captureRequests('delete', '*/api/genagents/v1/bots/:token', noContent)
    const rotated = captureRequests('post', '*/api/genagents/v1/bots/:token/secret', noContent)
    setAccessToken('token-1')

    await expect(listBots()).resolves.toEqual([bot])
    await listBots('reviewer')
    await createBot({
      name: 'Dev Bot', kind: 'dev', accountToken: 'account-1', model: 'claude-sonnet-5-5', role: 'role-1', effort: 'default', maxTurns: 100, timeoutMinutes: 60,
      skillTokens: [], mcpServerTokens: [],
    })
    await updateBot('bot-1', { instructions: null, enabled: false })
    await deleteBot('bot-1')
    await rotateBotSecret('bot-1')

    expect(listed[0]?.search).toBe('')
    expect(listed[1]?.search).toBe('?kind=reviewer')
    expect((created[0]?.body as { accountToken: string }).accountToken).toBe('account-1')
    expect(patched[0]?.body).toEqual({ instructions: null, enabled: false })
    expect(deleted[0]?.path).toBe('/api/genagents/v1/bots/bot-1')
    expect(rotated[0]?.path).toBe('/api/genagents/v1/bots/bot-1/secret')
  })
})

describe('skills and MCP servers', () => {
  it('manages skills and imports a SKILL.md', async () => {
    const skill = skillFixture()
    captureRequests('get', '*/api/genagents/v1/skills', () => HttpResponse.json({ skills: [skill] }))
    const created = captureRequests('post', '*/api/genagents/v1/skills', () => HttpResponse.json(skill, { status: 201 }))
    const imported = captureRequests('post', '*/api/genagents/v1/skills/import', () => HttpResponse.json(skill, { status: 201 }))
    const patched = captureRequests('patch', '*/api/genagents/v1/skills/:token', () => HttpResponse.json(skill))
    const deleted = captureRequests('delete', '*/api/genagents/v1/skills/:token', noContent)
    setAccessToken('token-1')

    await expect(listSkills()).resolves.toEqual([skill])
    await createSkill({ name: 'go-conventions', description: 'd', body: 'b', files: [] })
    await importSkill('---\nname: x\ndescription: y\n---\nbody')
    await updateSkill('skill-1', { files: [] })
    await deleteSkill('skill-1')

    expect(created[0]?.body).toEqual({ name: 'go-conventions', description: 'd', body: 'b', files: [] })
    expect(imported[0]?.body).toEqual({ content: '---\nname: x\ndescription: y\n---\nbody' })
    expect(patched[0]?.body).toEqual({ files: [] })
    expect(deleted[0]?.path).toBe('/api/genagents/v1/skills/skill-1')
  })

  it('manages MCP servers', async () => {
    const server = mcpServerFixture()
    captureRequests('get', '*/api/genagents/v1/mcp-servers', () => HttpResponse.json({ servers: [server] }))
    const created = captureRequests('post', '*/api/genagents/v1/mcp-servers', () => HttpResponse.json(server, { status: 201 }))
    const patched = captureRequests('patch', '*/api/genagents/v1/mcp-servers/:token', () => HttpResponse.json(server))
    const deleted = captureRequests('delete', '*/api/genagents/v1/mcp-servers/:token', noContent)
    setAccessToken('token-1')

    await expect(listMcpServers()).resolves.toEqual([server])
    await createMcpServer({ name: 'sentry', transport: 'http', url: 'https://x.io', headers: [{ name: 'Authorization', secret: true, value: 'Bearer s' }] })
    await updateMcpServer('server-1', { url: 'https://y.io' })
    await deleteMcpServer('server-1')

    expect((created[0]?.body as { headers: unknown[] }).headers).toHaveLength(1)
    expect(patched[0]?.body).toEqual({ url: 'https://y.io' })
    expect(deleted[0]?.path).toBe('/api/genagents/v1/mcp-servers/server-1')
  })
})

describe('runners', () => {
  it('lists, creates, revokes, and removes runners', async () => {
    const runner = runnerFixture()
    captureRequests('get', '*/api/genagents/v1/runners', () => HttpResponse.json({ runners: [runner] }))
    const created = captureRequests('post', '*/api/genagents/v1/runners', () => HttpResponse.json({ ...runner, registrationToken: 'registration-token', registrationExpiresAt: '2026-11-05T13:00:00Z' }, { status: 201 }))
    const revoked = captureRequests('delete', '*/api/genagents/v1/runners/:token', noContent)
    const removed = captureRequests('post', '*/api/genagents/v1/runners/:token/removal', noContent)
    setAccessToken('token-1')

    await expect(listRunners()).resolves.toEqual([runner])
    await expect(createRunner('build-box')).resolves.toMatchObject({ registrationToken: 'registration-token' })
    await revokeRunner('runner-1')
    await removeRunner('runner/1')

    expect(created[0]?.body).toEqual({ name: 'build-box' })
    expect(revoked[0]?.path).toBe('/api/genagents/v1/runners/runner-1')
    expect(removed[0]?.path).toBe('/api/genagents/v1/runners/runner%2F1/removal')
  })
})

describe('admin chats', () => {
  it('starts, reads incrementally, and manages a chat', async () => {
    const chat = { token: 'chat-1', title: 'Ops', botName: 'Ops Assistant', entries: [], lastSequence: 0, activeRun: null, updatedAt: '2026-11-05T12:00:00Z' }
    captureRequests('get', '*/api/genagents/v1/admin-chats', () => HttpResponse.json({ chats: [{ token: 'chat-1', title: 'Ops', updatedAt: chat.updatedAt }] }))
    const started = captureRequests('post', '*/api/genagents/v1/admin-chats', () => HttpResponse.json(chat, { status: 201 }))
    const read = captureRequests('get', '*/api/genagents/v1/admin-chats/:token', () => HttpResponse.json(chat))
    const renamed = captureRequests('patch', '*/api/genagents/v1/admin-chats/:token', noContent)
    const deleted = captureRequests('delete', '*/api/genagents/v1/admin-chats/:token', noContent)
    const messaged = captureRequests('post', '*/api/genagents/v1/admin-chats/:token/messages', () => new HttpResponse(null, { status: 202 }))
    const stopped = captureRequests('post', '*/api/genagents/v1/admin-chats/:token/stop', noContent)
    const reopened = captureRequests('post', '*/api/genagents/v1/admin-chats/:token/reopen', noContent)
    setAccessToken('token-1')

    await expect(listChats()).resolves.toHaveLength(1)
    await startChat('bot-1', 'Ops')
    await expect(readChat('chat-1', 40)).resolves.toMatchObject({ botName: 'Ops Assistant' })
    await renameChat('chat-1', 'Renamed')
    await sendChatMessage('chat-1', 'list my tasks')
    await stopChat('chat-1')
    await reopenChat('chat-1')
    await deleteChat('chat-1')

    expect(started[0]?.body).toEqual({ botToken: 'bot-1', title: 'Ops' })
    expect(read[0]?.search).toBe('?after=40')
    expect(renamed[0]?.body).toEqual({ title: 'Renamed' })
    expect(messaged[0]?.body).toEqual({ text: 'list my tasks' })
    expect(stopped[0]?.path).toBe('/api/genagents/v1/admin-chats/chat-1/stop')
    expect(reopened[0]?.path).toBe('/api/genagents/v1/admin-chats/chat-1/reopen')
    expect(deleted[0]?.path).toBe('/api/genagents/v1/admin-chats/chat-1')
  })

  it('maps another admin chat to a 404', async () => {
    captureRequests('get', '*/api/genagents/v1/admin-chats/:token', () => errorResponse(404, 'chat_not_found'))
    setAccessToken('token-1')

    await expect(readChat('chat-9', 0)).rejects.toMatchObject({ status: 404, code: 'chat_not_found' })
  })
})

describe('account sign-ins', () => {
  it('starts a sign-in, submits the pasted code, and cancels', async () => {
    const signIn = signInFixture({ state: 'awaiting_user', url: 'https://claude.ai/oauth/authorize', urlHost: 'claude.ai', expectsCode: true })
    const started = captureRequests('post', '*/api/genagents/v1/accounts/:token/sign-ins', () => HttpResponse.json(signIn, { status: 201 }))
    const submitted = captureRequests('post', '*/api/genagents/v1/accounts/:token/sign-ins/:signIn/code', () =>
      HttpResponse.json({ ...signIn, state: 'code_submitted' }),
    )
    const cancelled = captureRequests('delete', '*/api/genagents/v1/accounts/:token/sign-ins/:signIn', noContent)
    setAccessToken('token-1')

    await expect(startAccountSignIn('account/1')).resolves.toEqual(signIn)
    await expect(submitAccountSignInCode('account-1', 'sign-in-1', 'code#state')).resolves.toMatchObject({ state: 'code_submitted' })
    await cancelAccountSignIn('account-1', 'sign-in-1')

    expect(started[0]?.path).toBe('/api/genagents/v1/accounts/account%2F1/sign-ins')
    expect(started[0]?.body).toBeUndefined()
    expect(submitted[0]?.path).toBe('/api/genagents/v1/accounts/account-1/sign-ins/sign-in-1/code')
    expect(submitted[0]?.body).toEqual({ code: 'code#state' })
    expect(cancelled[0]?.path).toBe('/api/genagents/v1/accounts/account-1/sign-ins/sign-in-1')
  })

  it('surfaces an expired sign-in as its error code', async () => {
    captureRequests('post', '*/api/genagents/v1/accounts/:token/sign-ins/:signIn/code', () => errorResponse(409, 'sign_in_not_active'))
    setAccessToken('token-1')

    await expect(submitAccountSignInCode('account-1', 'sign-in-1', 'late')).rejects.toMatchObject({ code: 'sign_in_not_active' })
  })
})

describe('MCP server OAuth', () => {
  it('starts an authorization and disconnects', async () => {
    const started = captureRequests('post', '*/api/genagents/v1/mcp-servers/:token/oauth/authorizations', () =>
      HttpResponse.json({ authorizationUrl: 'https://auth.example/authorize?state=s' }, { status: 201 }),
    )
    const disconnected = captureRequests('delete', '*/api/genagents/v1/mcp-servers/:token/oauth', noContent)
    setAccessToken('token-1')

    await expect(startMcpOAuth('server-1')).resolves.toBe('https://auth.example/authorize?state=s')
    await startMcpOAuth('server-1', { clientId: 'manual', clientSecret: 'secret' })
    await disconnectMcpOAuth('server-1')

    expect(started[0]?.body).toEqual({})
    expect(started[1]?.body).toEqual({ clientId: 'manual', clientSecret: 'secret' })
    expect(disconnected[0]?.path).toBe('/api/genagents/v1/mcp-servers/server-1/oauth')
  })

  it('surfaces a server that needs a client id', async () => {
    captureRequests('post', '*/api/genagents/v1/mcp-servers/:token/oauth/authorizations', () => errorResponse(409, 'oauth_client_id_required'))
    setAccessToken('token-1')

    await expect(startMcpOAuth('server-1')).rejects.toMatchObject({ code: 'oauth_client_id_required' })
  })
})
