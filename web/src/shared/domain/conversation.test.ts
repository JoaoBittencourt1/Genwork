import { describe, expect, it } from 'vitest'
import {
  conversationRefreshInterval,
  entryMessageKey,
  entryParams,
  entryText,
  isRunActive,
  isValidMessage,
  lastSequenceOf,
  mergeEntries,
  type ConversationEntry,
} from './conversation'

function entry(sequence: number, payload: Record<string, unknown> = {}): ConversationEntry {
  return { sequence, type: 'status', payload, authorToken: null, runToken: null, createdAt: '2026-11-05T12:00:00Z' }
}

describe('conversationRefreshInterval', () => {
  const run = (state: 'running' | 'succeeded') => ({ token: 'r', kind: 'task' as const, state, botName: 'Dev Bot' })

  it('polls every 2 seconds while a run is active and every 10 seconds otherwise', () => {
    expect(conversationRefreshInterval({ activeRun: run('running') })).toBe(2_000)
    expect(conversationRefreshInterval({ activeRun: null })).toBe(10_000)
    expect(conversationRefreshInterval({ activeRun: run('succeeded') })).toBe(10_000)
  })

  it('stops while the tab is hidden', () => {
    expect(conversationRefreshInterval({ activeRun: run('running'), hidden: true })).toBe(false)
  })
})

describe('isRunActive', () => {
  it('counts queued, running, and waiting runs as active', () => {
    expect(['queued', 'running', 'waiting_for_input'].every((state) => isRunActive(state as never))).toBe(true)
    expect(['succeeded', 'failed', 'cancelled'].some((state) => isRunActive(state as never))).toBe(false)
  })
})

describe('mergeEntries', () => {
  it('orders by sequence and keeps a repeated sequence once', () => {
    const shown = [entry(1), entry(2, { code: 'old' })]

    const merged = mergeEntries(shown, [entry(4), entry(2, { code: 'new' }), entry(3)])

    expect(merged.map((item) => item.sequence)).toEqual([1, 2, 3, 4])
    expect(merged[1]?.payload.code).toBe('new')
  })

  it('returns the same list when nothing arrived, so nothing renders again', () => {
    const shown = [entry(1)]

    expect(mergeEntries(shown, [])).toBe(shown)
  })

  it('finds where the next read starts', () => {
    expect(lastSequenceOf([entry(3), entry(9), entry(5)])).toBe(9)
    expect(lastSequenceOf([])).toBe(0)
  })
})

describe('isValidMessage', () => {
  it('accepts 1 to 8000 characters once trimmed', () => {
    expect(isValidMessage('hello')).toBe(true)
    expect(isValidMessage('   ')).toBe(false)
    expect(isValidMessage('x'.repeat(8000))).toBe(true)
    expect(isValidMessage('x'.repeat(8001))).toBe(false)
  })
})

describe('entry text', () => {
  it('maps a code to its message key and reads its parameters', () => {
    const failed = entry(1, { code: 'run_failed', reason: 'timeout', params: { message: 'slow' } })

    expect(entryMessageKey(failed)).toBe('conversation.entry.run_failed')
    expect(entryParams(failed)).toEqual({ message: 'slow', reason: 'timeout' })
    expect(entryMessageKey(entry(2))).toBe('conversation.entry.unknown')
    expect(entryParams(entry(3, { code: 'run_queued' }))).toEqual({})
  })

  it('shows the reason and the message of a failure the runner reported', () => {
    const error = (payload: Record<string, unknown>): ConversationEntry => ({ ...entry(4, payload), type: 'error' })
    const posted = error({ reason: 'github_post_failed', message: 'github answered 422: Unprocessable Entity: Line could not be resolved' })

    expect(entryMessageKey(posted)).toBe('conversation.entry.run_failed_with_message')
    expect(entryParams(posted)).toEqual({ reason: 'github_post_failed', message: 'github answered 422: Unprocessable Entity: Line could not be resolved' })
    expect(entryMessageKey(error({ reason: 'timeout' }))).toBe('conversation.entry.run_failed')
    expect(entryMessageKey(error({ reason: 'account_error', message: 'invalid key' }))).toBe('conversation.entry.run_failed')
    expect(entryMessageKey(error({ message: 'no reason' }))).toBe('conversation.entry.unknown')
    expect(entryMessageKey(entry(5, { reason: 'timeout', message: 'status entries keep their code' }))).toBe('conversation.entry.unknown')
  })

  it('reads the text of a message', () => {
    expect(entryText(entry(1, { text: 'hi' }))).toBe('hi')
    expect(entryText(entry(2))).toBe('')
  })
})
