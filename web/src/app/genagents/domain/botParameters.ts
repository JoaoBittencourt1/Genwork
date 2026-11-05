export const MAX_TURNS_RANGE = { min: 1, max: 500 } as const
export const TIMEOUT_MINUTES_RANGE = { min: 5, max: 720 } as const
export const DEFAULT_MAX_TURNS = 100
export const DEFAULT_TIMEOUT_MINUTES = 60

export function isValidMaxTurns(value: number): boolean {
  return Number.isInteger(value) && value >= MAX_TURNS_RANGE.min && value <= MAX_TURNS_RANGE.max
}

export function isValidTimeoutMinutes(value: number): boolean {
  return Number.isInteger(value) && value >= TIMEOUT_MINUTES_RANGE.min && value <= TIMEOUT_MINUTES_RANGE.max
}

export type BotFormProblems = Partial<Record<'name' | 'accountToken' | 'model' | 'role' | 'maxTurns' | 'timeoutMinutes', 'required' | 'invalid'>>

type BotFormFields = { name: string; accountToken: string; model: string; role: string; maxTurns: number; timeoutMinutes: number }

// botFormProblems is what the form checks before it sends anything; the API checks again.
export function botFormProblems(fields: BotFormFields): BotFormProblems {
  const problems: BotFormProblems = {}
  if (fields.name.trim() === '') problems.name = 'required'
  if (fields.accountToken === '') problems.accountToken = 'required'
  if (fields.model === '') problems.model = 'required'
  if (fields.role === '') problems.role = 'required'
  if (!isValidMaxTurns(fields.maxTurns)) problems.maxTurns = 'invalid'
  if (!isValidTimeoutMinutes(fields.timeoutMinutes)) problems.timeoutMinutes = 'invalid'
  return problems
}
