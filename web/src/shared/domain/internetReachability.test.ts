import { describe, expect, it } from 'vitest'
import { isReachableFromInternet } from './internetReachability'

describe('internet reachability', () => {
  it('accepts public https hosts', () => {
    expect(isReachableFromInternet('https://genwork.acme.com/api/gentasks/v1/github/webhook')).toBe(true)
    expect(isReachableFromInternet('https://203.0.113.10/api/gentasks/v1/github/webhook')).toBe(true)
    expect(isReachableFromInternet('https://172.32.0.1/hook')).toBe(true)
  })

  it('warns about plain http', () => {
    expect(isReachableFromInternet('http://genwork.acme.com/api/gentasks/v1/github/webhook')).toBe(false)
    expect(isReachableFromInternet('http://localhost:8485')).toBe(false)
  })

  it('accepts a bare origin', () => {
    expect(isReachableFromInternet('https://genwork.example.com')).toBe(true)
  })

  it('warns about local and private hosts', () => {
    for (const url of [
      'https://localhost:8485/hook',
      'https://genwork.localhost/hook',
      'https://127.0.0.1/hook',
      'https://127.8.9.10/hook',
      'https://0.0.0.0/hook',
      'https://10.1.2.3/hook',
      'https://172.16.0.1/hook',
      'https://172.31.255.255/hook',
      'https://192.168.1.20/hook',
      'https://169.254.10.10/hook',
      'https://[::1]/hook',
      'https://[fd12:3456::1]/hook',
      'https://[fe80::1]/hook',
    ]) {
      expect(isReachableFromInternet(url), url).toBe(false)
    }
  })

  it('warns about malformed URLs', () => {
    expect(isReachableFromInternet('not a url')).toBe(false)
    expect(isReachableFromInternet('')).toBe(false)
  })
})
