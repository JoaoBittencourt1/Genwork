import { apiRequest } from '@/shared/datasource/httpClient'
import type { Label, LabelColor, LabelPatch } from '../domain/types'

export async function listLabels(boardToken: string): Promise<Label[]> {
  return (await apiRequest<{ labels: Label[] }>('GET', `/api/gentasks/v1/boards/${encodeURIComponent(boardToken)}/labels`)).labels
}

export function createLabel(boardToken: string, name: string, color: LabelColor): Promise<Label> {
  return apiRequest<Label>('POST', `/api/gentasks/v1/boards/${encodeURIComponent(boardToken)}/labels`, { body: { name, color } })
}

export function updateLabel(labelToken: string, patch: LabelPatch): Promise<Label> {
  return apiRequest<Label>('PATCH', `/api/gentasks/v1/labels/${encodeURIComponent(labelToken)}`, { body: patch })
}

export function deleteLabel(labelToken: string): Promise<void> {
  return apiRequest<void>('DELETE', `/api/gentasks/v1/labels/${encodeURIComponent(labelToken)}`)
}
