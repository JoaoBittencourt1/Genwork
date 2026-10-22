import { describe, expect, it } from 'vitest'
import { ApiError, NETWORK_ERROR_CODE } from '@/shared/datasource/httpClient'
import {
  availableUpdateFixture,
  developmentFixture,
  failedFixture,
  succeededFixture,
  updateStatusFixture,
  updatingFixture,
  upToDateFixture,
} from '@/test/systemFixtures'
import {
  canCheckForUpdates,
  deriveUpdateCardView,
  isNewerVersion,
  isRestartingError,
  POLL_INTERVAL_MS,
  type UpdateCardContext,
} from './updateCard'

const now = Date.parse('2026-10-22T14:00:00Z')
const quiet: UpdateCardContext = { isChecking: false, isConfirming: false, followedUpdate: false, lastPollError: null, unreachableSince: null, now }

describe('deriveUpdateCardView', () => {
  it('shows the installed version before any check', () => {
    const view = deriveUpdateCardView(updateStatusFixture(), quiet)

    expect(view).toMatchObject({ kind: 'idle', installedVersion: '0.5.0', checkedAt: null, nextPollDelay: null, shouldReloadApp: false })
  })

  it('shows a busy state while checking', () => {
    expect(deriveUpdateCardView(updateStatusFixture(), { ...quiet, isChecking: true }).kind).toBe('checking')
  })

  it('says the installation is up to date', () => {
    expect(deriveUpdateCardView(upToDateFixture, quiet)).toMatchObject({ kind: 'upToDate', checkedAt: '2026-10-22T13:55:00Z' })
  })

  it('offers the newer release with its notes', () => {
    expect(deriveUpdateCardView(availableUpdateFixture, quiet)).toMatchObject({
      kind: 'available',
      latest: { version: '0.5.2', notesUrl: 'https://github.com/acme/genwork/releases/tag/v0.5.2' },
    })
  })

  it('asks for confirmation before updating', () => {
    expect(deriveUpdateCardView(availableUpdateFixture, { ...quiet, isConfirming: true })).toMatchObject({ kind: 'confirming', latest: { version: '0.5.2' } })
  })

  it('follows a running update and keeps polling', () => {
    expect(deriveUpdateCardView(updatingFixture, quiet)).toMatchObject({
      kind: 'updating',
      step: 'migrate',
      targetVersion: '0.5.2',
      nextPollDelay: POLL_INTERVAL_MS,
    })
  })

  it.each([
    ['a network error', new ApiError(0, NETWORK_ERROR_CODE)],
    ['a 502', new ApiError(502, 'unknown_error')],
    ['a 503', new ApiError(503, 'updater_unavailable')],
  ])('counts %s during an update as restarting', (_, error) => {
    const view = deriveUpdateCardView(updatingFixture, { ...quiet, followedUpdate: true, lastPollError: error, unreachableSince: now - 1000 })

    expect(view).toMatchObject({ kind: 'restarting', nextPollDelay: POLL_INTERVAL_MS })
  })

  it('reloads the app once the update it followed succeeded', () => {
    expect(deriveUpdateCardView(succeededFixture, { ...quiet, followedUpdate: true })).toMatchObject({
      kind: 'succeeded',
      installedVersion: '0.5.2',
      shouldReloadApp: true,
      nextPollDelay: null,
    })
  })

  it('does not reload again after the reload, when the update finished before this page opened', () => {
    expect(deriveUpdateCardView(succeededFixture, quiet)).toMatchObject({ kind: 'upToDate', shouldReloadApp: false })
  })

  it('reports a failure and whether the previous version is back', () => {
    expect(deriveUpdateCardView(failedFixture(true), quiet)).toMatchObject({ kind: 'failed', failure: { step: 'migrate', rolledBack: true } })
    expect(deriveUpdateCardView(failedFixture(false), quiet)).toMatchObject({ kind: 'failed', failure: { code: 'migration_failed', rolledBack: false } })
  })

  it('gives up after 15 minutes without an answer', () => {
    const view = deriveUpdateCardView(updatingFixture, {
      ...quiet,
      followedUpdate: true,
      lastPollError: new ApiError(0, NETWORK_ERROR_CODE),
      unreachableSince: now - 15 * 60 * 1000,
    })

    expect(view).toMatchObject({ kind: 'gaveUp', nextPollDelay: null })
  })

  it('explains why a development checkout cannot update', () => {
    expect(deriveUpdateCardView(developmentFixture, quiet)).toMatchObject({ kind: 'notUpdatable', reason: 'development_installation' })
  })

  it('shows the loading state until the first status answers', () => {
    expect(deriveUpdateCardView(undefined, quiet)).toMatchObject({ kind: 'loading', nextPollDelay: null })
  })

  it('keeps polling after a restart error when no status was read yet', () => {
    expect(deriveUpdateCardView(undefined, { ...quiet, followedUpdate: true, lastPollError: new ApiError(502, 'unknown_error'), unreachableSince: now })).toMatchObject({
      kind: 'restarting',
    })
  })
})

describe('isRestartingError', () => {
  it('tells restarts from real failures', () => {
    expect(isRestartingError(new ApiError(0, NETWORK_ERROR_CODE))).toBe(true)
    expect(isRestartingError(new ApiError(504, 'unknown_error'))).toBe(true)
    expect(isRestartingError(new ApiError(409, 'update_in_progress'))).toBe(false)
    expect(isRestartingError(new Error('boom'))).toBe(false)
    expect(isRestartingError(null)).toBe(false)
  })
})

describe('isNewerVersion', () => {
  it('compares semantic versions', () => {
    expect(isNewerVersion('0.5.2', '0.5.0')).toBe(true)
    expect(isNewerVersion('0.10.0', '0.9.9')).toBe(true)
    expect(isNewerVersion('1.0.0', '0.99.0')).toBe(true)
    expect(isNewerVersion('0.5.0', '0.5.0')).toBe(false)
    expect(isNewerVersion('0.5.0', '0.5.2')).toBe(false)
    expect(isNewerVersion('0.5.0', '0.5.0-dev')).toBe(true)
    expect(isNewerVersion('next', '0.5.0')).toBe(false)
  })
})

describe('canCheckForUpdates', () => {
  it('offers a check at rest and after a failure, never during an update or in a checkout', () => {
    expect(canCheckForUpdates(deriveUpdateCardView(availableUpdateFixture, quiet))).toBe(true)
    expect(canCheckForUpdates(deriveUpdateCardView(failedFixture(true), quiet))).toBe(true)
    expect(canCheckForUpdates(deriveUpdateCardView(updatingFixture, quiet))).toBe(false)
    expect(canCheckForUpdates(deriveUpdateCardView(developmentFixture, quiet))).toBe(false)
  })
})
