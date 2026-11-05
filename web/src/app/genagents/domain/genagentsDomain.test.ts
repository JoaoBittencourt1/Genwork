import { describe, expect, it } from 'vitest'
import { accountFixture, botFixture, runnerFixture } from '@/test/genagentsFixtures'
import { botFormProblems, isValidMaxTurns, isValidTimeoutMinutes } from './botParameters'
import { buildBotPatch, draftFromBot, isEmptyPatch } from './botPatch'
import { canSeeAiSection, GENAGENTS_PERMISSIONS, genagentsAbilities } from './permissions'
import {
  accountSignInHint,
  botsOfKind,
  botsUsingAccount,
  firstSupportedMode,
  isAccountUsable,
  keepModelOrFirst,
  modelsOfAccount,
  modesFor,
  supportsMode,
} from './providers'
import { runnerRegisterCommand, runnersWithTool, runnerStatus, runnerWarnings } from './runners'
import { addSkillFile, isValidSkillFilePath, isValidSkillName, MAX_SKILL_FILES, parseSkillMarkdown } from './skills'
import type { ProviderCatalogEntry } from './types'

const catalog: ProviderCatalogEntry[] = [
  { provider: 'anthropic', cli: 'claude', modes: ['subscription', 'api_key'], models: [] },
  { provider: 'xai', cli: 'grok', modes: ['api_key'], models: [] },
]

describe('provider mode rules', () => {
  it('offers xAI only API keys', () => {
    expect(modesFor(catalog, 'xai')).toEqual(['api_key'])
    expect(supportsMode(catalog, 'xai', 'subscription')).toBe(false)
    expect(supportsMode(catalog, 'anthropic', 'subscription')).toBe(true)
    expect(modesFor(catalog, 'cursor')).toEqual([])
  })

  it('moves the form to a supported mode when the provider changes', () => {
    expect(firstSupportedMode(catalog, 'xai', 'subscription')).toBe('api_key')
    expect(firstSupportedMode(catalog, 'anthropic', 'subscription')).toBe('subscription')
    expect(firstSupportedMode(catalog, 'cursor', 'subscription')).toBe('subscription')
  })
})

describe('model filtering by account', () => {
  const accounts = [
    accountFixture({ token: 'a1', models: [{ token: null, id: 'claude-sonnet-5-5', displayName: 'Sonnet', custom: false }] }),
    accountFixture({ token: 'a2', provider: 'openai', models: [{ token: null, id: 'gpt-5', displayName: 'GPT-5', custom: false }] }),
  ]

  it('lists the models of the chosen account only', () => {
    expect(modelsOfAccount(accounts, 'a2').map((model) => model.id)).toEqual(['gpt-5'])
    expect(modelsOfAccount(accounts, 'missing')).toEqual([])
  })

  it('keeps the model when the new account offers it, and resets it otherwise', () => {
    expect(keepModelOrFirst(accounts, 'a1', 'claude-sonnet-5-5')).toBe('claude-sonnet-5-5')
    expect(keepModelOrFirst(accounts, 'a2', 'claude-sonnet-5-5')).toBe('gpt-5')
    expect(keepModelOrFirst(accounts, 'missing', 'x')).toBe('')
  })
})

describe('accounts and bots', () => {
  it('shows the sign-in command only while an account waits for it', () => {
    const pending = accountFixture({ status: 'pending_sign_in', signInCommand: 'genwork-runner account login a1' })

    expect(accountSignInHint(pending)).toBe('genwork-runner account login a1')
    expect(accountSignInHint(accountFixture())).toBeNull()
    expect(isAccountUsable(pending)).toBe(false)
    expect(isAccountUsable(accountFixture())).toBe(true)
  })

  it('names the bots that keep an account from being removed', () => {
    const bots = [botFixture({ name: 'A' }), botFixture({ token: 'b2', name: 'B', account: { token: 'other', name: 'Other' } })]

    expect(botsUsingAccount(bots, 'account-1')).toEqual(['A'])
  })

  it('lists enabled bots of a kind', () => {
    const bots = [botFixture({ name: 'R1', kind: 'reviewer' }), botFixture({ name: 'R2', kind: 'reviewer', enabled: false }), botFixture({ name: 'D', kind: 'dev' })]

    expect(botsOfKind(bots, 'reviewer').map((bot) => bot.name)).toEqual(['R1'])
  })
})

describe('permissions', () => {
  it('derives the abilities from the permissions held', () => {
    const abilities = genagentsAbilities((permission) => permission === GENAGENTS_PERMISSIONS.botManage || permission === GENAGENTS_PERMISSIONS.adminChatUse)

    expect(abilities.canManageBots).toBe(true)
    expect(abilities.canUseAdminChat).toBe(true)
    expect(abilities.canManageAccounts).toBe(false)
    expect(canSeeAiSection(abilities)).toBe(true)
    expect(canSeeAiSection(genagentsAbilities(() => false))).toBe(false)
  })
})

describe('bot parameters', () => {
  it('accepts 1–500 turns and 5–720 minutes', () => {
    expect(isValidMaxTurns(1)).toBe(true)
    expect(isValidMaxTurns(500)).toBe(true)
    expect(isValidMaxTurns(0)).toBe(false)
    expect(isValidMaxTurns(501)).toBe(false)
    expect(isValidMaxTurns(2.5)).toBe(false)
    expect(isValidTimeoutMinutes(5)).toBe(true)
    expect(isValidTimeoutMinutes(720)).toBe(true)
    expect(isValidTimeoutMinutes(4)).toBe(false)
  })

  it('lists what the form still needs', () => {
    expect(botFormProblems({ name: ' ', accountToken: '', model: '', role: '', maxTurns: 0, timeoutMinutes: 1 })).toEqual({
      name: 'required', accountToken: 'required', model: 'required', role: 'required', maxTurns: 'invalid', timeoutMinutes: 'invalid',
    })
    expect(botFormProblems({ name: 'Dev', accountToken: 'a', model: 'm', role: 'r', maxTurns: 10, timeoutMinutes: 10 })).toEqual({})
  })
})

describe('buildBotPatch', () => {
  it('sends nothing when nothing changed', () => {
    const bot = botFixture()

    expect(isEmptyPatch(buildBotPatch(bot, draftFromBot(bot)))).toBe(true)
  })

  it('sends only the changed field, so saving one never rewrites the others (rule 17)', () => {
    const bot = botFixture({ instructions: 'Be careful' })

    expect(buildBotPatch(bot, { ...draftFromBot(bot), model: 'claude-opus-5-5' })).toEqual({ model: 'claude-opus-5-5' })
    expect(buildBotPatch(bot, { ...draftFromBot(bot), enabled: false })).toEqual({ enabled: false })
  })

  it('clears instructions with null and attaches skills regardless of order', () => {
    const bot = botFixture({ instructions: 'Be careful', skills: [{ token: 's1', name: 'a' }, { token: 's2', name: 'b' }] })

    expect(buildBotPatch(bot, { ...draftFromBot(bot), instructions: '  ' })).toEqual({ instructions: null })
    expect(buildBotPatch(bot, { ...draftFromBot(bot), skillTokens: ['s2', 's1'] })).toEqual({})
    expect(buildBotPatch(bot, { ...draftFromBot(bot), skillTokens: ['s1'] })).toEqual({ skillTokens: ['s1'] })
    expect(buildBotPatch(bot, { ...draftFromBot(bot), mcpServerTokens: ['m1'], name: ' Renamed ' })).toEqual({ name: 'Renamed', mcpServerTokens: ['m1'] })
  })
})

describe('runner status', () => {
  it('derives the status shown on the Runners page', () => {
    expect(runnerStatus(runnerFixture())).toBe('online')
    expect(runnerStatus(runnerFixture({ online: false }))).toBe('offline')
    expect(runnerStatus(runnerFixture({ revoked: true }))).toBe('revoked')
    expect(runnerStatus(runnerFixture({ registered: false, online: false }))).toBe('pending')
  })

  it('warns about directory mode, missing GitHub credentials, and custom images', () => {
    expect(runnerWarnings(runnerFixture())).toEqual([])
    expect(runnerWarnings(runnerFixture({ directoryMode: true, hasGithubCredentials: false, customImage: true }))).toEqual(['directoryMode', 'noGithubCredentials', 'customImage'])
    expect(runnerWarnings(runnerFixture({ registered: false, hasGithubCredentials: false }))).toEqual([])
  })

  it('builds the register command and finds runners with a CLI', () => {
    expect(runnerRegisterCommand('https://genwork.example.com', 'tok')).toBe('genwork-runner register --instance https://genwork.example.com --token tok')
    const runners = [runnerFixture({ token: 'a' }), runnerFixture({ token: 'b', tools: {} }), runnerFixture({ token: 'c', revoked: true })]

    expect(runnersWithTool(runners, 'claude').map((runner) => runner.token)).toEqual(['a'])
  })
})

describe('skills', () => {
  it('accepts lowercase names with digits and hyphens up to 64 characters', () => {
    expect(isValidSkillName('go-conventions')).toBe(true)
    expect(isValidSkillName('Go')).toBe(false)
    expect(isValidSkillName('a'.repeat(65))).toBe(false)
    expect(isValidSkillName('')).toBe(false)
  })

  it('rejects file paths that leave the skill directory', () => {
    for (const path of ['', 'SKILL.md', '/abs', 'a/../b', 'a//b', './a', 'a\\b']) expect(isValidSkillFilePath(path)).toBe(false)
    expect(isValidSkillFilePath('examples/a.go')).toBe(true)
  })

  it('adds files up to the limit without duplicates', () => {
    let files = addSkillFile([], ' examples/a.go ')
    expect(files).toEqual([{ path: 'examples/a.go', content: '' }])
    expect(addSkillFile(files, 'examples/a.go')).toBe(files)
    expect(addSkillFile(files, '../x')).toBe(files)
    for (let index = 0; index < MAX_SKILL_FILES; index++) files = addSkillFile(files, `f${index}`)
    expect(files).toHaveLength(MAX_SKILL_FILES)
  })

  it('reads the front matter of a pasted SKILL.md', () => {
    expect(parseSkillMarkdown('---\nname: pr-review\ndescription: "Reviews pull requests"\n---\n\n# Steps\n')).toEqual({
      name: 'pr-review', description: 'Reviews pull requests', body: '# Steps\n',
    })
    expect(parseSkillMarkdown('no front matter')).toBeNull()
    expect(parseSkillMarkdown('---\nname: x\n---\nbody')).toBeNull()
    expect(parseSkillMarkdown('---\nname: x\ndescription: y\nbody')).toBeNull()
  })
})
