import type { TFunction } from 'i18next'
import { ApiError } from '../api/client'

export function describeError(t: TFunction, error: unknown): string {
  const code = error instanceof ApiError ? error.code : 'unknown_error'
  return t(`errors.${code}`, { defaultValue: t('errors.unknown_error') })
}

export function describeFieldError(t: TFunction, error: unknown, field: string): string | undefined {
  if (!(error instanceof ApiError)) return undefined
  const reason = error.fields?.[field]
  return reason ? t(`errors.fields.${reason}`, { defaultValue: t('errors.fields.invalid') }) : undefined
}
