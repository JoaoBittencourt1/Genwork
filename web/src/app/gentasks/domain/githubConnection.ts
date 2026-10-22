import { ApiError } from '@/shared/datasource/httpClient'
import { errorTranslationKey } from '@/shared/domain/errors'

export type ManifestCallback = { code: string; state: string }

// readManifestCallback reads the one-time code and state GitHub appends when it redirects back after creating the App.
export function readManifestCallback(search: string): ManifestCallback | null {
  const params = new URLSearchParams(search)
  const code = params.get('code')
  const state = params.get('state')
  return code && state ? { code, state } : null
}

export function githubConnectionErrorKey(error: unknown): string {
  if (error instanceof ApiError && error.fields?.state) return 'gentasks.github.errors.stateInvalid'
  if (error instanceof ApiError && error.fields?.code) return 'gentasks.github.errors.codeInvalid'
  return errorTranslationKey(error)
}
