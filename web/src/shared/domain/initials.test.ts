import { describe, expect, it } from 'vitest'
import { initials } from './initials'

describe('initials', () => {
  it('takes the first and last word of a name, or the first two letters of a single word', () => {
    expect(initials('Ana Souza')).toBe('AS')
    expect(initials('Ana Maria Souza')).toBe('AS')
    expect(initials('Bruno')).toBe('BR')
  })
})
