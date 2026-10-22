import { apiRequest } from '@/shared/datasource/httpClient'
import type { HostSnapshot } from '../domain/hostSnapshot'

const HOST_PATH = '/api/genmonitor/v1/host'

export function readHostSnapshot(): Promise<HostSnapshot> {
  return apiRequest<HostSnapshot>('GET', HOST_PATH)
}
