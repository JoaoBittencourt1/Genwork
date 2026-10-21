import { afterAll, afterEach, beforeAll } from 'vitest'
import { resetApiClientState } from '@/shared/datasource/httpClient'
import { server } from './server'

beforeAll(() => server.listen({ onUnhandledFrame: 'error' }))
afterEach(() => {
  server.resetHandlers()
  resetApiClientState()
})
afterAll(() => server.close())
