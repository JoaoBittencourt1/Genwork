// Conversation types shared by the genagents screens and the gentasks task dialog, which both read the same entries (rule 7).

export const RUN_STATES = ['queued', 'running', 'waiting_for_input', 'succeeded', 'failed', 'cancelled'] as const
export type RunState = (typeof RUN_STATES)[number]

export const RUN_KINDS = ['task', 'review', 'chat'] as const
export type RunKind = (typeof RUN_KINDS)[number]

export const ENTRY_TYPES = ['human_message', 'status', 'assistant_message', 'tool_use', 'result', 'error'] as const
export type EntryType = (typeof ENTRY_TYPES)[number]

// ConversationEntry payloads carry message keys with parameters, not copy, so each screen localizes them.
export type ConversationEntry = {
  sequence: number
  type: EntryType
  payload: Record<string, unknown>
  authorToken: string | null
  runToken: string | null
  createdAt: string
}

export type ActiveRun = {
  token: string
  kind: RunKind
  state: RunState
  botName: string
}

export type ConversationPage = {
  entries: ConversationEntry[]
  lastSequence: number
  activeRun: ActiveRun | null
}

export const ACTIVE_RUN_REFRESH_MS = 2_000
export const IDLE_REFRESH_MS = 10_000

export function isRunActive(state: RunState): boolean {
  return state === 'queued' || state === 'running' || state === 'waiting_for_input'
}

// conversationRefreshInterval polls fast only while a run is active, and stops while the tab is hidden.
export function conversationRefreshInterval({ activeRun, hidden = false }: { activeRun: ActiveRun | null; hidden?: boolean }): number | false {
  if (hidden) return false
  return activeRun && isRunActive(activeRun.state) ? ACTIVE_RUN_REFRESH_MS : IDLE_REFRESH_MS
}

// mergeEntries adds an incremental read to what is shown: ordered by sequence, with a repeated sequence kept once.
export function mergeEntries(shown: ConversationEntry[], incoming: ConversationEntry[]): ConversationEntry[] {
  if (incoming.length === 0) return shown
  const bySequence = new Map<number, ConversationEntry>()
  for (const entry of shown) bySequence.set(entry.sequence, entry)
  for (const entry of incoming) bySequence.set(entry.sequence, entry)
  return [...bySequence.values()].sort((left, right) => left.sequence - right.sequence)
}

// lastSequenceOf is where the next incremental read starts.
export function lastSequenceOf(entries: ConversationEntry[]): number {
  return entries.reduce((last, entry) => Math.max(last, entry.sequence), 0)
}

export const MAX_MESSAGE_LENGTH = 8000

export function isValidMessage(text: string): boolean {
  const trimmed = text.trim()
  return trimmed.length > 0 && [...trimmed].length <= MAX_MESSAGE_LENGTH
}

// entryMessageKey names the localized text of a status or error entry; an unknown code falls back to a generic key.
export function entryMessageKey(entry: ConversationEntry): string {
  const code = typeof entry.payload.code === 'string' ? entry.payload.code : null
  if (code === null) return 'conversation.entry.unknown'
  return `conversation.entry.${code}`
}

export function entryParams(entry: ConversationEntry): Record<string, unknown> {
  const params = entry.payload.params
  const base = params && typeof params === 'object' ? (params as Record<string, unknown>) : {}
  return typeof entry.payload.reason === 'string' ? { ...base, reason: entry.payload.reason } : base
}

export function entryText(entry: ConversationEntry): string {
  return typeof entry.payload.text === 'string' ? entry.payload.text : ''
}
