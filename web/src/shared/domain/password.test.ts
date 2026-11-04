import { describe, expect, it } from 'vitest'
import { passwordsMatch } from './password'

describe('passwordsMatch', () => {
  it('requires the exact same text', () => {
    expect(passwordsMatch('correct horse', 'correct horse')).toBe(true)
    expect(passwordsMatch('correct horse', 'correct Horse')).toBe(false)
    expect(passwordsMatch('correct horse', 'correct horse ')).toBe(false)
  })
})
