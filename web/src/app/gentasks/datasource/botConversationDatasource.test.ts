import { HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { setAccessToken } from '@/shared/datasource/httpClient'
import { captureRequests, errorResponse } from '@/test/server'
import { clearTaskConversation, listBotOptions, postTaskMessage, readTaskConversation, requestBotReview, stopTaskRun } from './botConversationDatasource'

describe('botConversationDatasource', () => {
  it('lists the bots a task can be assigned to or reviewed by', async () => {
    captureRequests('get', '*/api/genagents/v1/bots', () =>
      HttpResponse.json({ bots: [{ token: 'bot-1', name: 'Dev Bot', kind: 'dev', memberToken: 'member-1', enabled: true, model: 'x', account: {} }] }),
    )
    setAccessToken('token-1')

    await expect(listBotOptions()).resolves.toEqual([{ token: 'bot-1', memberToken: 'member-1', name: 'Dev Bot', kind: 'dev', enabled: true }])
  })

  it('reads the conversation after a sequence and posts, stops, and requests reviews', async () => {
    const page = { entries: [], lastSequence: 0, activeRun: null }
    const read = captureRequests('get', '*/api/genagents/v1/tasks/:token/conversation', () => HttpResponse.json(page))
    const posted = captureRequests('post', '*/api/genagents/v1/tasks/:token/conversation/messages', () => new HttpResponse(null, { status: 202 }))
    const stopped = captureRequests('post', '*/api/genagents/v1/tasks/:token/conversation/stop', () => new HttpResponse(null, { status: 204 }))
    const cleared = captureRequests('delete', '*/api/genagents/v1/tasks/:token/conversation', () => new HttpResponse(null, { status: 204 }))
    const reviewed = captureRequests('post', '*/api/genagents/v1/tasks/:token/bot-reviews', () => HttpResponse.json({ runToken: 'run-1', state: 'queued' }, { status: 202 }))
    setAccessToken('token-1')

    await expect(readTaskConversation('task/1', 40)).resolves.toEqual(page)
    await expect(postTaskMessage('task-1', 'use the repository')).resolves.toBeUndefined()
    await expect(stopTaskRun('task-1')).resolves.toBeUndefined()
    await expect(requestBotReview('task-1')).resolves.toEqual({ runToken: 'run-1', state: 'queued' })
    await requestBotReview('task-1', 'bot-9')

    expect(read[0]?.path).toBe('/api/genagents/v1/tasks/task%2F1/conversation')
    expect(read[0]?.search).toBe('?after=40')
    expect(posted[0]?.body).toEqual({ text: 'use the repository' })
    expect(stopped[0]?.path).toBe('/api/genagents/v1/tasks/task-1/conversation/stop')
    expect(reviewed[0]?.path).toBe('/api/genagents/v1/tasks/task-1/bot-reviews')
    await clearTaskConversation('task/1')
    expect(cleared[0]?.path).toBe('/api/genagents/v1/tasks/task%2F1/conversation')
    expect(reviewed[0]?.body).toEqual({})
    expect(reviewed[1]?.body).toEqual({ botToken: 'bot-9' })
  })

  it('maps the review refusals', async () => {
    captureRequests('post', '*/api/genagents/v1/tasks/:token/bot-reviews', () => errorResponse(409, 'no_open_pull_requests'))
    setAccessToken('token-1')

    await expect(requestBotReview('task-1')).rejects.toMatchObject({ status: 409, code: 'no_open_pull_requests' })
  })
})
