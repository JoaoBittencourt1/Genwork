import { describe, expect, it } from 'vitest'
import { AVATAR_COLORS, avatarColorVariable } from './avatarColor'

describe('avatarColorVariable', () => {
  it('maps every avatar color to its theme variable', () => {
    expect(AVATAR_COLORS.map(avatarColorVariable)).toEqual([
      'var(--avatar-blue)',
      'var(--avatar-light-green)',
      'var(--avatar-pink)',
      'var(--avatar-purple)',
      'var(--avatar-red)',
      'var(--avatar-yellow)',
      'var(--avatar-white)',
      'var(--avatar-orange)',
    ])
  })
})
