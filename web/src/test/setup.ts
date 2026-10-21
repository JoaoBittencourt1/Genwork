import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterAll, afterEach, beforeAll } from 'vitest'
import { resetApiClientState } from '../shared/api/client'
import { setUpI18n } from '../shared/i18n'
import { server } from './server'

setUpI18n('pt-BR')

beforeAll(() => server.listen({ onUnhandledFrame: 'error' }))
afterEach(() => {
  cleanup()
  server.resetHandlers()
  resetApiClientState()
})
afterAll(() => server.close())
