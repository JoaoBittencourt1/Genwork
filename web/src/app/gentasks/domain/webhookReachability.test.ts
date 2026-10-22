import { describe, expect, it } from 'vitest'
import { isWebhookReachableByGithub } from './webhookReachability'

describe('webhook reachability', () => {
  it('accepts public https hosts', () => {
    expect(isWebhookReachableByGithub('https://genwork.acme.com/api/gentasks/v1/github/webhook')).toBe(true)
    expect(isWebhookReachableByGithub('https://203.0.113.10/api/gentasks/v1/github/webhook')).toBe(true)
    expect(isWebhookReachableByGithub('https://172.32.0.1/hook')).toBe(true)
  })

  it('warns about plain http', () => {
    expect(isWebhookReachableByGithub('http://genwork.acme.com/api/gentasks/v1/github/webhook')).toBe(false)
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
      expect(isWebhookReachableByGithub(url), url).toBe(false)
    }
  })

  it('warns about malformed URLs', () => {
    expect(isWebhookReachableByGithub('not a url')).toBe(false)
    expect(isWebhookReachableByGithub('')).toBe(false)
  })
})
