import { useTranslation } from 'react-i18next'
import {
  errorTranslationKey,
  fallbackErrorTranslationKey,
  fallbackFieldErrorTranslationKey,
  fieldErrorTranslationKey,
} from '@/shared/domain/errors'

export function useErrorMessage() {
  const { t } = useTranslation()
  return {
    describeError: (error: unknown) => t(errorTranslationKey(error), { defaultValue: t(fallbackErrorTranslationKey) }),
    describeFieldError: (error: unknown, field: string) => {
      const key = fieldErrorTranslationKey(error, field)
      return key ? t(key, { defaultValue: t(fallbackFieldErrorTranslationKey) }) : undefined
    },
  }
}
