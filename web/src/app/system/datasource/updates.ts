import { apiRequest } from '@/shared/datasource/httpClient'
import type { UpdateStatus } from '../domain/updateCard'

export function findUpdateStatus(): Promise<UpdateStatus> {
  return apiRequest<UpdateStatus>('GET', '/api/core/v1/system/update')
}

export function checkForUpdates(): Promise<UpdateStatus> {
  return apiRequest<UpdateStatus>('POST', '/api/core/v1/system/update/check')
}

// startUpdate names the version the admin confirmed; core refuses it with version_mismatch if a newer one appeared.
export function startUpdate(version: string): Promise<UpdateStatus> {
  return apiRequest<UpdateStatus>('POST', '/api/core/v1/system/update', { body: { version } })
}
