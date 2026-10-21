// Mirrors the server rule: an uppercase letter, then up to nine uppercase letters or digits.
const TASK_PREFIX_PATTERN = /^[A-Z][A-Z0-9]{0,9}$/

export const MAX_TASK_PREFIX_LENGTH = 10

export function taskIdentifier(taskPrefix: string, taskNumber: number): string {
  return `${taskPrefix}-${taskNumber}`
}

export function normalizeTaskPrefix(prefix: string): string {
  return prefix.trim().toUpperCase()
}

export function isValidTaskPrefix(normalizedPrefix: string): boolean {
  return TASK_PREFIX_PATTERN.test(normalizedPrefix)
}
