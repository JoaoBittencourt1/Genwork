import { describe, expect, it } from 'vitest'
import { GENWORK_COMMANDS, genworkCommandLine } from './cliCommands'

describe('genworkCommandLine', () => {
  it('prefixes every command with the CLI name', () => {
    expect(GENWORK_COMMANDS.map(genworkCommandLine)).toEqual([
      'genwork up',
      'genwork db',
      'genwork api',
      'genwork web',
      'genwork auth',
      'genwork modules',
      'genwork down',
    ])
  })
})
