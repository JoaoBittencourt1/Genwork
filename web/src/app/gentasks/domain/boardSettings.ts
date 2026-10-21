import { isValidTaskPrefix, normalizeTaskPrefix } from './taskPrefix'
import type { BoardPatch, BoardSummary } from './types'

export const MAX_BOARD_NAME_LENGTH = 80

export type BoardSettingsDraft = {
  name: string
  taskPrefix: string
}

export function draftFromBoard(board: BoardSummary): BoardSettingsDraft {
  return { name: board.name, taskPrefix: board.taskPrefix }
}

// buildBoardPatch sends only what changed after normalizing, so saving an untouched form changes nothing (rule 17).
export function buildBoardPatch(board: BoardSummary, draft: BoardSettingsDraft): BoardPatch {
  const patch: BoardPatch = {}
  const name = draft.name.trim()
  const taskPrefix = normalizeTaskPrefix(draft.taskPrefix)
  if (name !== board.name) patch.name = name
  if (taskPrefix !== board.taskPrefix) patch.taskPrefix = taskPrefix
  return patch
}

export function isBoardSettingsDraftValid(draft: BoardSettingsDraft): boolean {
  const nameLength = draft.name.trim().length
  return nameLength > 0 && nameLength <= MAX_BOARD_NAME_LENGTH && isValidTaskPrefix(normalizeTaskPrefix(draft.taskPrefix))
}
