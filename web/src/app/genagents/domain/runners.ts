import type { Runner } from './types'

export type RunnerStatus = 'online' | 'offline' | 'revoked' | 'pending'

// runnerStatus derives what the Runners page shows from the fields the API reports.
export function runnerStatus(runner: Runner): RunnerStatus {
  if (runner.revoked) return 'revoked'
  if (!runner.registered) return 'pending'
  return runner.online ? 'online' : 'offline'
}

// runnerCanBeRemoved is true once the runner can never work again: it was revoked, or it never registered and the
// one-hour registration token has expired. A live registration is cancelled with revoke, not removed.
export function runnerCanBeRemoved(runner: Runner, now: Date): boolean {
  if (runner.revoked) return true
  if (runner.registered || runner.registrationExpiresAt == null) return false
  return new Date(runner.registrationExpiresAt).getTime() <= now.getTime()
}

export type RunnerWarning = 'directoryMode' | 'noGithubCredentials' | 'customImage'

// runnerWarnings lists what an admin should notice about a runner: unsafe directory mode, missing GitHub credentials
// (it cannot take task runs or reviews), and a custom sandbox image.
export function runnerWarnings(runner: Runner): RunnerWarning[] {
  const warnings: RunnerWarning[] = []
  if (runner.directoryMode) warnings.push('directoryMode')
  if (runner.registered && !runner.hasGithubCredentials) warnings.push('noGithubCredentials')
  if (runner.customImage) warnings.push('customImage')
  return warnings
}

// runnerRegisterCommand is what the admin runs on the runner's machine within an hour of creating the runner.
export function runnerRegisterCommand(instanceUrl: string, registrationToken: string): string {
  return `genwork-runner register --instance ${instanceUrl} --token ${registrationToken}`
}

export function runnersWithTool(runners: Runner[], cli: string): Runner[] {
  return runners.filter((runner) => !runner.revoked && runner.registered && cli in runner.tools)
}
