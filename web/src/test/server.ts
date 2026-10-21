import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'

export const server = setupServer(
  http.post('*/api/core/v1/auth/refresh', () => HttpResponse.json({ error: { code: 'invalid_token' } }, { status: 401 })),
)

export function errorResponse(status: number, code: string, fields?: Record<string, string>) {
  return HttpResponse.json({ error: { code, fields } }, { status })
}

export type CapturedRequest = {
  method: string
  path: string
  headers: Headers
  body: unknown
}

// captureRequests records every call to the given path pattern and answers with the provided body.
export function captureRequests(method: 'get' | 'post' | 'delete', pattern: string, respond: () => Response): CapturedRequest[] {
  const captured: CapturedRequest[] = []
  server.use(
    http[method](pattern, async ({ request }) => {
      const text = await request.text()
      captured.push({
        method: request.method,
        path: new URL(request.url).pathname,
        headers: request.headers,
        body: text ? JSON.parse(text) : undefined,
      })
      return respond()
    }),
  )
  return captured
}
