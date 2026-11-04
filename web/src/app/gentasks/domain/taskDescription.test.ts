import { describe, expect, it } from 'vitest'
import { isDescriptionBlank } from './taskDescription'

describe('task description', () => {
  it('is blank when empty or only whitespace', () => {
    expect(isDescriptionBlank('')).toBe(true)
    expect(isDescriptionBlank(' ')).toBe(true)
    expect(isDescriptionBlank('\n\t ')).toBe(true)
  })

  it('is not blank when it has any visible character', () => {
    expect(isDescriptionBlank('## Title')).toBe(false)
    expect(isDescriptionBlank(' a ')).toBe(false)
  })
})
