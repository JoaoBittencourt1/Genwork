import type { Account, AccountMode, Bot, BotKind, Provider, ProviderCatalogEntry, ProviderModel } from './types'

// modesFor lists the account modes a provider supports; xAI has no official subscription sign-in for a CLI.
export function modesFor(catalog: ProviderCatalogEntry[], provider: Provider): AccountMode[] {
  return catalog.find((entry) => entry.provider === provider)?.modes ?? []
}

export function supportsMode(catalog: ProviderCatalogEntry[], provider: Provider, mode: AccountMode): boolean {
  return modesFor(catalog, provider).includes(mode)
}

// firstSupportedMode keeps the account form on a valid mode when the provider changes.
export function firstSupportedMode(catalog: ProviderCatalogEntry[], provider: Provider, current: AccountMode): AccountMode {
  const modes = modesFor(catalog, provider)
  return modes.includes(current) ? current : (modes[0] ?? current)
}

// modelsOfAccount is what a bot on the account may use: its provider's catalog plus the account's custom models.
export function modelsOfAccount(accounts: Account[], accountToken: string): ProviderModel[] {
  return accounts.find((account) => account.token === accountToken)?.models ?? []
}

// keepModelOrFirst resets the chosen model when a new account does not offer it.
export function keepModelOrFirst(accounts: Account[], accountToken: string, model: string): string {
  const models = modelsOfAccount(accounts, accountToken)
  return models.some((candidate) => candidate.id === model) ? model : (models[0]?.id ?? '')
}

export function isAccountUsable(account: Account): boolean {
  return account.status === 'ready'
}

// accountSignInHint is the runner command that signs a pending subscription account in.
export function accountSignInHint(account: Account): string | null {
  return account.status === 'pending_sign_in' ? account.signInCommand : null
}

// botsUsingAccount names the bots that keep an account from being removed.
export function botsUsingAccount(bots: Bot[], accountToken: string): string[] {
  return bots.filter((bot) => bot.account.token === accountToken).map((bot) => bot.name)
}

export function botsOfKind(bots: Bot[], kind: BotKind): Bot[] {
  return bots.filter((bot) => bot.kind === kind && bot.enabled)
}
