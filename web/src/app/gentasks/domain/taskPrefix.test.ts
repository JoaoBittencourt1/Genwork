import { describe, expect, it } from 'vitest'
import { isValidTaskPrefix, normalizeTaskPrefix, taskIdentifier } from './taskPrefix'

describe('task prefix', () => {
  it('joins the board prefix and the task number', () => {
    expect(taskIdentifier('VAN', 128)).toBe('VAN-128')
  })

  it('normalizes to trimmed uppercase', () => {
    expect(normalizeTaskPrefix('  van1 ')).toBe('VAN1')
  })

  it('accepts a letter followed by up to nine letters or digits', () => {
    expect(isValidTaskPrefix('V')).toBe(true)
    expect(isValidTaskPrefix('VAN2026')).toBe(true)
    expect(isValidTaskPrefix('A'.repeat(10))).toBe(true)
    expect(isValidTaskPrefix('A'.repeat(11))).toBe(false)
    expect(isValidTaskPrefix('')).toBe(false)
    expect(isValidTaskPrefix('1AB')).toBe(false)
    expect(isValidTaskPrefix('A-B')).toBe(false)
    expect(isValidTaskPrefix('van')).toBe(false)
  })
})
