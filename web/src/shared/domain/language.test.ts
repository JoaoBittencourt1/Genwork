import { describe, expect, it } from 'vitest'
import { toSupportedLanguage } from './language'

describe('toSupportedLanguage', () => {
  it.each([
    ['en', 'en'],
    ['en-US', 'en'],
    ['EN-gb', 'en'],
    ['pt-BR', 'pt-BR'],
    ['pt-PT', 'pt-BR'],
    ['fr-FR', 'pt-BR'],
    ['', 'pt-BR'],
  ])('maps %s to %s', (detected, expected) => {
    expect(toSupportedLanguage(detected)).toBe(expected)
  })
})
