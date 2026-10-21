import { describe, expect, it } from 'vitest'
import { ApiError } from '@/shared/datasource/httpClient'
import { errorTranslationKey, fieldErrorTranslationKey } from './errors'

describe('errorTranslationKey', () => {
  it('maps API error codes to translation keys', () => {
    expect(errorTranslationKey(new ApiError(401, 'invalid_credentials'))).toBe('errors.invalid_credentials')
  })

  it('falls back to the unknown error key for anything else', () => {
    expect(errorTranslationKey(new Error('boom'))).toBe('errors.unknown_error')
    expect(errorTranslationKey(undefined)).toBe('errors.unknown_error')
  })
})

describe('fieldErrorTranslationKey', () => {
  it('maps a field reason to its translation key', () => {
    const error = new ApiError(400, 'validation_failed', { email: 'invalid', deviceName: 'too_long' })
    expect(fieldErrorTranslationKey(error, 'email')).toBe('errors.fields.invalid')
    expect(fieldErrorTranslationKey(error, 'deviceName')).toBe('errors.fields.too_long')
  })

  it('returns nothing for fields without errors or non-API errors', () => {
    expect(fieldErrorTranslationKey(new ApiError(400, 'validation_failed', { email: 'invalid' }), 'name')).toBeUndefined()
    expect(fieldErrorTranslationKey(new ApiError(401, 'invalid_credentials'), 'email')).toBeUndefined()
    expect(fieldErrorTranslationKey(new Error('boom'), 'email')).toBeUndefined()
  })
})
