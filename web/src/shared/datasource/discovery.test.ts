import { HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { captureRequests } from '@/test/server'
import { fetchDiscovery } from './discovery'
import { setAccessToken } from './httpClient'

describe('fetchDiscovery', () => {
  it('reads the discovery document without credentials', async () => {
    const document = { product: 'genwork', name: 'Genwork da Empresa', version: '0.1.0', api: 'v1', auth: { methods: ['password'] } }
    const requests = captureRequests('get', '*/.well-known/genwork', () => HttpResponse.json(document))
    setAccessToken('token-1')

    await expect(fetchDiscovery()).resolves.toEqual(document)
    expect(requests[0]?.headers.get('Authorization')).toBeNull()
  })
})
