// The commands of cli/internal/commands/root.go in the order a new installation uses them; update both together.
export const GENWORK_COMMANDS = ['up', 'db', 'api', 'web', 'auth', 'modules', 'down'] as const
export type GenworkCommand = (typeof GENWORK_COMMANDS)[number]

export function genworkCommandLine(command: GenworkCommand): string {
  return `genwork ${command}`
}
