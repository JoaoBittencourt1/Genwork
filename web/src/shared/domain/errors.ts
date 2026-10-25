import { ApiError } from '@/shared/datasource/httpClient'

const UNKNOWN_ERROR_CODE = 'unknown_error'
const INVALID_FIELD_REASON = 'invalid'

export function errorTranslationKey(error: unknown): string {
  const code = error instanceof ApiError ? error.code : UNKNOWN_ERROR_CODE
  return `errors.${code}`
}

export function fieldErrorTranslationKey(error: unknown, field: string): string | undefined {
  if (!(error instanceof ApiError)) return undefined
  const reason = error.fields?.[field]
  return reason ? `errors.fields.${reason}` : undefined
}

export const fallbackErrorTranslationKey = `errors.${UNKNOWN_ERROR_CODE}`
export const fallbackFieldErrorTranslationKey = `errors.fields.${INVALID_FIELD_REASON}`

// A failed background refresh keeps the data already on screen, so only a failed first load is shown as an error.
export function loadError(query: { isLoadingError: boolean; error: unknown }): unknown {
  return query.isLoadingError ? query.error : null
}
