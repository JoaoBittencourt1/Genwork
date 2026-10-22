import type { UpdateStatus } from '@/app/system/domain/updateCard'

const latestRelease = { version: '0.5.2', notesUrl: 'https://github.com/acme/genwork/releases/tag/v0.5.2', publishedAt: '2026-10-20T12:00:00Z' }

// updateStatusFixture is what core relays from the updater; each test changes only what it is about.
export function updateStatusFixture(overrides: Partial<UpdateStatus> = {}): UpdateStatus {
  return { installedVersion: '0.5.0', updatable: true, latest: null, checkedAt: null, state: 'idle', ...overrides }
}

export const availableUpdateFixture = updateStatusFixture({ latest: latestRelease, checkedAt: '2026-10-22T13:55:00Z' })

export const upToDateFixture = updateStatusFixture({ installedVersion: '0.5.2', latest: latestRelease, checkedAt: '2026-10-22T13:55:00Z' })

export const updatingFixture = updateStatusFixture({
  latest: latestRelease,
  checkedAt: '2026-10-22T13:55:00Z',
  state: 'updating',
  targetVersion: '0.5.2',
  step: 'migrate',
  startedAt: '2026-10-22T13:56:00Z',
})

export const succeededFixture = updateStatusFixture({
  installedVersion: '0.5.2',
  latest: latestRelease,
  checkedAt: '2026-10-22T13:55:00Z',
  state: 'succeeded',
  targetVersion: '0.5.2',
  step: 'verify',
  startedAt: '2026-10-22T13:56:00Z',
  finishedAt: '2026-10-22T13:58:00Z',
})

export function failedFixture(rolledBack: boolean): UpdateStatus {
  return updateStatusFixture({
    latest: latestRelease,
    checkedAt: '2026-10-22T13:55:00Z',
    state: 'failed',
    targetVersion: '0.5.2',
    step: 'migrate',
    startedAt: '2026-10-22T13:56:00Z',
    finishedAt: '2026-10-22T13:58:00Z',
    failure: { code: 'migration_failed', step: 'migrate', rolledBack },
  })
}

export const developmentFixture = updateStatusFixture({ updatable: false, reason: 'development_installation' })
