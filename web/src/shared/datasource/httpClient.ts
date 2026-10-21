export type ErrorFields = Record<string, string>

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly fields?: ErrorFields

  constructor(status: number, code: string, fields?: ErrorFields) {
    super(code)
    this.status = status
    this.code = code
    this.fields = fields
  }
}

export const NETWORK_ERROR_CODE = 'network_error'
const CLIENT_HEADER = 'X-Genwork-Client'

type RequestOptions = {
  body?: unknown
  authenticated?: boolean
}

type SessionEndedListener = () => void

// The access token lives only in memory; the refresh token is an HttpOnly cookie the browser sends to /api/core/v1/auth.
let accessToken: string | null = null
let pendingRefresh: Promise<boolean> | null = null
const sessionEndedListeners = new Set<SessionEndedListener>()

export function setAccessToken(token: string | null): void {
  accessToken = token
}

export function hasAccessToken(): boolean {
  return accessToken !== null
}

export function onSessionEnded(listener: SessionEndedListener): () => void {
  sessionEndedListeners.add(listener)
  return () => sessionEndedListeners.delete(listener)
}

export function resetApiClientState(): void {
  accessToken = null
  pendingRefresh = null
  sessionEndedListeners.clear()
}

export async function apiRequest<T>(method: string, path: string, options: RequestOptions = {}): Promise<T> {
  const authenticated = options.authenticated ?? true
  let response = await send(method, path, options.body, authenticated)
  if (authenticated && response.status === 401 && (await readErrorCode(response.clone())) === 'invalid_token') {
    if (!(await refreshSession())) {
      endSession()
      throw new ApiError(401, 'invalid_token')
    }
    response = await send(method, path, options.body, authenticated)
  }
  return parseResponse<T>(response)
}

// Concurrent callers share one refresh so a rotated refresh token is never presented twice.
export function refreshSession(): Promise<boolean> {
  pendingRefresh ??= performRefresh().finally(() => {
    pendingRefresh = null
  })
  return pendingRefresh
}

export function endSession(): void {
  accessToken = null
  sessionEndedListeners.forEach((listener) => listener())
}

async function performRefresh(): Promise<boolean> {
  try {
    const response = await send('POST', '/api/core/v1/auth/refresh', undefined, false)
    if (!response.ok) {
      accessToken = null
      return false
    }
    const body = (await response.json()) as { accessToken: string }
    accessToken = body.accessToken
    return true
  } catch {
    accessToken = null
    return false
  }
}

async function send(method: string, path: string, body: unknown, authenticated: boolean): Promise<Response> {
  const headers: Record<string, string> = { [CLIENT_HEADER]: 'web' }
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }
  if (authenticated && accessToken) {
    headers.Authorization = `Bearer ${accessToken}`
  }
  try {
    return await fetch(new URL(path, window.location.origin), {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      credentials: 'same-origin',
    })
  } catch {
    throw new ApiError(0, NETWORK_ERROR_CODE)
  }
}

async function parseResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const payload = await readErrorPayload(response)
    throw new ApiError(response.status, payload.code, payload.fields)
  }
  if (response.status === 204) {
    return undefined as T
  }
  return (await response.json()) as T
}

async function readErrorCode(response: Response): Promise<string> {
  return (await readErrorPayload(response)).code
}

async function readErrorPayload(response: Response): Promise<{ code: string; fields?: ErrorFields }> {
  try {
    const body = (await response.json()) as { error?: { code?: string; fields?: ErrorFields } }
    return { code: body.error?.code ?? 'unknown_error', fields: body.error?.fields }
  } catch {
    return { code: 'unknown_error' }
  }
}
