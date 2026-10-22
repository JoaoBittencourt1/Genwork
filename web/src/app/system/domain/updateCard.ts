import { ApiError, NETWORK_ERROR_CODE } from '@/shared/datasource/httpClient'

export type UpdateState = 'idle' | 'updating' | 'succeeded' | 'failed'
export type UpdateStep = 'download' | 'pull' | 'backup' | 'swap' | 'migrate' | 'verify'

export type LatestRelease = { version: string; notesUrl: string; publishedAt: string }
export type UpdateFailure = { code: string; step: UpdateStep; rolledBack: boolean }

// UpdateStatus is GET /api/core/v1/system/update, which core relays from the updater.
export type UpdateStatus = {
  installedVersion: string
  updatable: boolean
  reason?: string
  latest: LatestRelease | null
  checkedAt: string | null
  state: UpdateState
  targetVersion?: string
  step?: UpdateStep
  startedAt?: string
  finishedAt?: string
  failure?: UpdateFailure
}

export const POLL_INTERVAL_MS = 3000
export const GIVE_UP_AFTER_MS = 15 * 60 * 1000

// UpdateCardContext is what the card knows besides the status: the user's own actions and how polling went.
export type UpdateCardContext = {
  isChecking: boolean
  isConfirming: boolean
  // followedUpdate is true once this page saw the update running, so only that page reloads into the new version.
  followedUpdate: boolean
  lastPollError: unknown
  unreachableSince: number | null
  now: number
}

type ViewBase = { installedVersion: string | null; checkedAt: string | null; nextPollDelay: number | null; shouldReloadApp: boolean }

export type UpdateCardView = ViewBase &
  (
    | { kind: 'loading' | 'idle' | 'checking' | 'upToDate' | 'gaveUp' }
    | { kind: 'available' | 'confirming'; latest: LatestRelease }
    | { kind: 'updating'; step: UpdateStep | null; targetVersion: string | null }
    | { kind: 'restarting'; targetVersion: string | null }
    | { kind: 'succeeded' }
    | { kind: 'failed'; failure: UpdateFailure; targetVersion: string | null }
    | { kind: 'notUpdatable'; reason: string }
  )

// deriveUpdateCardView holds every decision of the update card (rule 74); the component only renders the view.
export function deriveUpdateCardView(status: UpdateStatus | undefined, context: UpdateCardContext): UpdateCardView {
  const base: ViewBase = {
    installedVersion: status?.installedVersion ?? null,
    checkedAt: status?.checkedAt ?? null,
    nextPollDelay: null,
    shouldReloadApp: false,
  }
  const restarting = isRestartingError(context.lastPollError) && (context.followedUpdate || status?.state === 'updating')
  if (restarting && context.unreachableSince !== null && context.now - context.unreachableSince >= GIVE_UP_AFTER_MS) {
    return { ...base, kind: 'gaveUp' }
  }
  if (restarting) return { ...base, kind: 'restarting', targetVersion: status?.targetVersion ?? null, nextPollDelay: POLL_INTERVAL_MS }
  if (!status) return { ...base, kind: 'loading' }
  if (!status.updatable) return { ...base, kind: 'notUpdatable', reason: status.reason ?? 'unknown' }
  switch (status.state) {
    case 'updating':
      return { ...base, kind: 'updating', step: status.step ?? null, targetVersion: status.targetVersion ?? null, nextPollDelay: POLL_INTERVAL_MS }
    case 'succeeded':
      if (context.followedUpdate && status.installedVersion === status.targetVersion) {
        return { ...base, kind: 'succeeded', shouldReloadApp: true }
      }
      break
    case 'failed':
      if (status.failure) return { ...base, kind: 'failed', failure: status.failure, targetVersion: status.targetVersion ?? null }
      break
  }
  return viewAtRest(base, status, context)
}

function viewAtRest(base: ViewBase, status: UpdateStatus, context: UpdateCardContext): UpdateCardView {
  if (context.isChecking) return { ...base, kind: 'checking' }
  if (status.latest && isNewerVersion(status.latest.version, status.installedVersion)) {
    return { ...base, kind: context.isConfirming ? 'confirming' : 'available', latest: status.latest }
  }
  return { ...base, kind: status.checkedAt ? 'upToDate' : 'idle' }
}

const CHECKABLE_VIEWS: UpdateCardView['kind'][] = ['idle', 'checking', 'upToDate', 'available', 'confirming', 'failed']

// canCheckForUpdates hides the check while an update runs, and where updates cannot be installed at all.
export function canCheckForUpdates(view: UpdateCardView): boolean {
  return CHECKABLE_VIEWS.includes(view.kind)
}

// isRestartingError is what polling sees while the gateway or core restart: no answer, or a proxy error.
export function isRestartingError(error: unknown): boolean {
  if (!(error instanceof ApiError)) return false
  return error.code === NETWORK_ERROR_CODE || error.status === 502 || error.status === 503 || error.status === 504
}

type ParsedVersion = { numbers: [number, number, number]; prerelease: boolean }

function parseVersion(version: string): ParsedVersion | null {
  const match = /^(\d+)\.(\d+)\.(\d+)(-[0-9A-Za-z.-]+)?$/.exec(version)
  if (!match) return null
  return { numbers: [Number(match[1]), Number(match[2]), Number(match[3])], prerelease: match[4] !== undefined }
}

// isNewerVersion compares semantic versions; a release is newer than a pre-release of the same numbers.
export function isNewerVersion(candidate: string, installed: string): boolean {
  const next = parseVersion(candidate)
  const current = parseVersion(installed)
  if (!next || !current) return false
  for (let index = 0; index < 3; index++) {
    if (next.numbers[index] !== current.numbers[index]) return next.numbers[index]! > current.numbers[index]!
  }
  return current.prerelease && !next.prerelease
}
