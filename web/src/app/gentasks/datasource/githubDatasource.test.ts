import { HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { setAccessToken } from '@/shared/datasource/httpClient'
import { boardGithubSettingsFixture, disconnectedGithubFixture, githubConnectionFixture } from '@/test/gentasksFixtures'
import { captureRequests, errorResponse } from '@/test/server'
import {
  completeGithubConnection,
  disconnectGithub,
  readBoardGithubSettings,
  readGithubConnection,
  startGithubManifest,
  updateBoardGithubSettings,
} from './githubDatasource'

const manifestStart = {
  formAction: 'https://github.com/organizations/acme/settings/apps/new?state=signed-state',
  manifest: { name: 'Genwork k3f9qz', public: false },
  state: 'signed-state',
}

describe('githubDatasource', () => {
  it('reads the connection', async () => {
    const read = captureRequests('get', '*/api/gentasks/v1/github/connection', () => HttpResponse.json(disconnectedGithubFixture))
    setAccessToken('token-1')

    await expect(readGithubConnection()).resolves.toEqual(disconnectedGithubFixture)

    expect(read[0]?.headers.get('Authorization')).toBe('Bearer token-1')
  })

  it('starts the manifest flow for an organization or the personal account', async () => {
    const started = captureRequests('post', '*/api/gentasks/v1/github/connection/manifest', () => HttpResponse.json(manifestStart))

    await expect(startGithubManifest('acme')).resolves.toEqual(manifestStart)
    await startGithubManifest(null)

    expect(started.map((request) => request.body)).toEqual([{ organization: 'acme' }, { organization: null }])
  })

  it('completes and removes the connection', async () => {
    const connection = githubConnectionFixture()
    const completed = captureRequests('post', '*/api/gentasks/v1/github/connection', () => HttpResponse.json(connection, { status: 201 }))
    const removed = captureRequests('delete', '*/api/gentasks/v1/github/connection', () => new HttpResponse(null, { status: 204 }))

    await expect(completeGithubConnection('one-time-code', 'signed-state')).resolves.toEqual(connection)
    await expect(disconnectGithub()).resolves.toBeUndefined()

    expect(completed[0]?.body).toEqual({ code: 'one-time-code', state: 'signed-state' })
    expect(removed).toHaveLength(1)
  })

  it('surfaces connection failures', async () => {
    const responses = [
      errorResponse(400, 'validation_failed', { state: 'invalid' }),
      errorResponse(400, 'validation_failed', { code: 'invalid' }),
      errorResponse(409, 'github_already_connected'),
      errorResponse(502, 'github_unavailable'),
    ]
    captureRequests('post', '*/api/gentasks/v1/github/connection', () => responses.shift() ?? errorResponse(500, 'internal_error'))

    await expect(completeGithubConnection('c', 's')).rejects.toMatchObject({ status: 400, fields: { state: 'invalid' } })
    await expect(completeGithubConnection('c', 's')).rejects.toMatchObject({ status: 400, fields: { code: 'invalid' } })
    await expect(completeGithubConnection('c', 's')).rejects.toMatchObject({ status: 409, code: 'github_already_connected' })
    await expect(completeGithubConnection('c', 's')).rejects.toMatchObject({ status: 502, code: 'github_unavailable' })
  })

  it('reads and patches board settings', async () => {
    const settings = boardGithubSettingsFixture({ reviewRequestedColumnToken: 'column-review' })
    const read = captureRequests('get', '*/api/gentasks/v1/boards/:token/github-settings', () => HttpResponse.json(settings))
    const patched = captureRequests('patch', '*/api/gentasks/v1/boards/:token/github-settings', () => HttpResponse.json(settings))

    await expect(readBoardGithubSettings('board/1')).resolves.toEqual(settings)
    await expect(updateBoardGithubSettings('board-1', { reviewRequestedColumnToken: 'column-review', mergedColumnToken: null })).resolves.toEqual(settings)

    expect(read[0]?.path).toBe('/api/gentasks/v1/boards/board%2F1/github-settings')
    expect(patched[0]?.body).toEqual({ reviewRequestedColumnToken: 'column-review', mergedColumnToken: null })
  })
})
