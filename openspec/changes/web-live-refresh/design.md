## Context

Every screen reads server data through TanStack Query. `App.tsx` creates the `QueryClient` with `retry: false` and `refetchOnWindowFocus: false`, so data only changes after the person's own mutations (which invalidate queries) or a reload. Two hooks already poll with their own cadence (`useHostSnapshot`, `useUpdateStatus`). The board applies moves and reassignments optimistically (`useBoardMoves`: cancel queries, `setQueryData`, roll back on error, invalidate on settle). Forms seed their drafts once from query data with `useState`, and the task dialog edits a task captured when it opened.

## Goals / Non-Goals

**Goals:** see other people's and the automation's changes within 10 seconds on every screen, without flicker, lost drafts, or error flashes.

**Non-Goals:** server push, conflict resolution, desktop clients (see proposal).

## Decisions

### 1. Polling as the `QueryClient` default
`refetchInterval: LIVE_REFRESH_INTERVAL_MS` (10 000) and `refetchOnWindowFocus: true` become `QueryClient` defaults; `refetchIntervalInBackground` stays `false`, so hidden tabs stop polling and focus triggers an immediate read. A default covers every current and future screen with no per-screen code (rule 7). Queries with their own `refetchInterval` already override it.

Alternative considered: a `useLiveQuery` wrapper used screen by screen — explicit, but every new screen would have to remember it, and the interval would be repeated at each call site.

`LIVE_REFRESH_INTERVAL_MS` lives in `web/src/shared/domain/liveRefresh.ts`, its only owner.

### 2. Explicit opt-outs
`refetchInterval: false` and `refetchOnWindowFocus: false` on: the invitation inspection (`AcceptInvitePage` — re-reading a secret-bearing endpoint is pointless, and an accepted invite would turn into an error), the assistant authorization preview (fixed for the OAuth request), and discovery (`useInstanceName`, already `staleTime` 5 minutes).

### 3. Pause the board while a move is saving
`onMutate` already cancels in-flight board reads, but a poll that starts after it and returns before the save completes would show the pre-move board for a moment. The board mutations get a shared mutation key per board; `BoardPage` reads `useIsMutating({ mutationKey })` and passes `refetchInterval: liveRefreshInterval({ savingChanges })`, which returns `false` while saving. TanStack re-evaluates the interval when the observer's options change, so polling resumes as soon as the count drops to zero; `onSettled` invalidation re-reads the board immediately.

The rule is a pure function in `shared/domain/liveRefresh.ts` and unit-tested (rule 74).

A global "pause while anything is mutating" function was rejected: TanStack only re-evaluates a function interval when *that* query updates, so a query not touched by the mutation would stay paused.

### 4. Background errors stay silent
TanStack keeps `data` when a refetch fails but sets `error`/`isError`. Screens that decide on `isError` or render `query.error` switch to TanStack's `isLoadingError` (error with no data), so only a failed first load shows the error. This is a per-screen markup change, listed in tasks.md from an audit of every `useQuery` consumer; mutation errors are untouched.

### 5. Drafts are already safe
The task dialog's draft is seeded once when it opens (`TaskForm` keyed by task token, patch built against the task captured at opening), so a refresh never resets it. The settings forms seed their drafts with `useState` and are keyed by the data they edit (`board.updatedAt`, which only changes when name or prefix are saved, and the GitHub settings themselves): a refresh that changed nothing they edit keeps the draft, and a refresh carrying someone else's save of the same form remounts it with the saved values. Keeping the stale draft instead would make the next save diff against the new values and revert the other person's change. `buildTaskPatch` sends only changed fields, so saving a dialog after someone else changed another field does not overwrite it. No code change is needed; the manual checklist covers it. Structural sharing in TanStack keeps unchanged data referentially equal, so memoized cards do not re-render on a refresh that changed nothing.

## Risks / Trade-offs

- [Load: each visible query is read every 10 s per person] → Reads are small indexed queries; hidden tabs stop. Measured during manual verification on a board with many tasks; the interval can change in one place.
- [A refresh lands mid-drag] → The canvas renders its own drag snapshot (`taskDrag.view`) while dragging, so the drag is not disturbed; the drop applies to the latest board optimistically.
- [Polling keeps the session refreshing while a tab is visible] → The same as using the app; the refresh token rotation (rule 69) and single shared refresh call already handle concurrent requests.
- [Two people edit the same field] → Last save wins, as today; conflict warnings are a non-goal.

## Migration Plan

Web-only; ships with the bundle. Rollback is reverting the change.

## Open Questions

None.
