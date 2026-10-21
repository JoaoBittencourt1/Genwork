export const SUPPORTED_LANGUAGES = ['pt-BR', 'en'] as const
export type SupportedLanguage = (typeof SUPPORTED_LANGUAGES)[number]

export const DEFAULT_LANGUAGE: SupportedLanguage = 'pt-BR'

export function toSupportedLanguage(detected: string): SupportedLanguage {
  return detected.trim().toLowerCase().startsWith('en') ? 'en' : DEFAULT_LANGUAGE
}
