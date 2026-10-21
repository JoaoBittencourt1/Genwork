import { apiRequest } from '@/shared/datasource/httpClient'
import type { BoardSummary, BoardView, Column, CreatedBoard } from '../domain/types'

const BOARDS_PATH = '/api/gentasks/v1/boards'
const COLUMNS_PATH = '/api/gentasks/v1/columns'

export async function listBoards(): Promise<BoardSummary[]> {
  return (await apiRequest<{ boards: BoardSummary[] }>('GET', BOARDS_PATH)).boards
}

export function createBoard(name: string): Promise<CreatedBoard> {
  return apiRequest<CreatedBoard>('POST', BOARDS_PATH, { body: { name } })
}

export function readBoard(boardToken: string): Promise<BoardView> {
  return apiRequest<BoardView>('GET', `${BOARDS_PATH}/${encodeURIComponent(boardToken)}`)
}

export function renameBoard(boardToken: string, name: string): Promise<BoardSummary> {
  return apiRequest<BoardSummary>('PATCH', `${BOARDS_PATH}/${encodeURIComponent(boardToken)}`, { body: { name } })
}

export function deleteBoard(boardToken: string): Promise<void> {
  return apiRequest<void>('DELETE', `${BOARDS_PATH}/${encodeURIComponent(boardToken)}`)
}

export function addColumn(boardToken: string, name: string): Promise<Column> {
  return apiRequest<Column>('POST', `${BOARDS_PATH}/${encodeURIComponent(boardToken)}/columns`, { body: { name } })
}

// renameColumn with null clears a custom name, so a preset column shows its localized label again.
export function renameColumn(columnToken: string, name: string | null): Promise<Column> {
  return apiRequest<Column>('PATCH', `${COLUMNS_PATH}/${encodeURIComponent(columnToken)}`, { body: { name } })
}

export function moveColumn(columnToken: string, position: number): Promise<void> {
  return apiRequest<void>('POST', `${COLUMNS_PATH}/${encodeURIComponent(columnToken)}/move`, { body: { position } })
}

export function deleteColumn(columnToken: string): Promise<void> {
  return apiRequest<void>('DELETE', `${COLUMNS_PATH}/${encodeURIComponent(columnToken)}`)
}
