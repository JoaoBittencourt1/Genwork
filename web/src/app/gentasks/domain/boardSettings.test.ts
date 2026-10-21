import { describe, expect, it } from 'vitest'
import { boardViewFixture } from '@/test/gentasksFixtures'
import { buildBoardPatch, draftFromBoard, isBoardSettingsDraftValid } from './boardSettings'

describe('board settings', () => {
  const board = boardViewFixture()

  it('sends nothing when nothing changed', () => {
    expect(buildBoardPatch(board, draftFromBoard(board))).toEqual({})
    expect(buildBoardPatch(board, { name: ' Platform ', taskPrefix: 'pla' })).toEqual({})
  })

  it('sends only the changed fields, normalized', () => {
    expect(buildBoardPatch(board, { name: 'Platform', taskPrefix: ' core ' })).toEqual({ taskPrefix: 'CORE' })
    expect(buildBoardPatch(board, { name: '  Renamed ', taskPrefix: 'PLA' })).toEqual({ name: 'Renamed' })
  })

  it('validates the name and the prefix', () => {
    expect(isBoardSettingsDraftValid({ name: 'Platform', taskPrefix: 'pla' })).toBe(true)
    expect(isBoardSettingsDraftValid({ name: '  ', taskPrefix: 'PLA' })).toBe(false)
    expect(isBoardSettingsDraftValid({ name: 'a'.repeat(81), taskPrefix: 'PLA' })).toBe(false)
    expect(isBoardSettingsDraftValid({ name: 'Platform', taskPrefix: '9' })).toBe(false)
  })
})
