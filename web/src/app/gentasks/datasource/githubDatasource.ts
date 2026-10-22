import { apiRequest } from '@/shared/datasource/httpClient'
import type { BoardGithubSettings, BoardGithubSettingsPatch, GithubConnection, GithubManifestStart } from '../domain/types'

const CONNECTION_PATH = '/api/gentasks/v1/github/connection'
const BOARDS_PATH = '/api/gentasks/v1/boards'

export function readGithubConnection(): Promise<GithubConnection> {
  return apiRequest<GithubConnection>('GET', CONNECTION_PATH)
}

// startGithubManifest with null creates the App under the admin's personal GitHub account.
export function startGithubManifest(organization: string | null): Promise<GithubManifestStart> {
  return apiRequest<GithubManifestStart>('POST', `${CONNECTION_PATH}/manifest`, { body: { organization } })
}

export function completeGithubConnection(code: string, state: string): Promise<GithubConnection> {
  return apiRequest<GithubConnection>('POST', CONNECTION_PATH, { body: { code, state } })
}

export function disconnectGithub(): Promise<void> {
  return apiRequest<void>('DELETE', CONNECTION_PATH)
}

function boardGithubSettingsPath(boardToken: string): string {
  return `${BOARDS_PATH}/${encodeURIComponent(boardToken)}/github-settings`
}

export function readBoardGithubSettings(boardToken: string): Promise<BoardGithubSettings> {
  return apiRequest<BoardGithubSettings>('GET', boardGithubSettingsPath(boardToken))
}

export function updateBoardGithubSettings(boardToken: string, patch: BoardGithubSettingsPatch): Promise<BoardGithubSettings> {
  return apiRequest<BoardGithubSettings>('PATCH', boardGithubSettingsPath(boardToken), { body: patch })
}
