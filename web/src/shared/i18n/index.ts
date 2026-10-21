import i18n from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'
import en from './locales/en.json'
import ptBR from './locales/pt-BR.json'

export const SUPPORTED_LANGUAGES = ['pt-BR', 'en'] as const

function toSupportedLanguage(detected: string): string {
  return detected.toLowerCase().startsWith('en') ? 'en' : 'pt-BR'
}

export function setUpI18n(language?: string) {
  if (i18n.isInitialized) {
    if (language) void i18n.changeLanguage(language)
    return i18n
  }
  void i18n
    .use(LanguageDetector)
    .use(initReactI18next)
    .init({
      resources: { 'pt-BR': { translation: ptBR }, en: { translation: en } },
      lng: language,
      fallbackLng: 'pt-BR',
      supportedLngs: SUPPORTED_LANGUAGES,
      detection: { order: ['navigator'], caches: [], convertDetectedLanguage: toSupportedLanguage },
      interpolation: { escapeValue: false },
      initAsync: false,
    })
  return i18n
}
