## Why

Screens in the web app only show what changed when someone else acts, or when the GitHub automation moves a task, after a manual page reload. On a shared board that means people work on stale cards, drag tasks that already moved, and miss new ones.

## What Changes

- Every server-backed screen refreshes its data automatically every 10 seconds while the browser tab is visible, and immediately when the person returns to the tab. Hidden tabs do not poll.
- The interval lives once in the web app (rule 7) and is the default for every query; screens that must not poll opt out explicitly:
  - accepting an invitation (the invite secret is inspected once),
  - the AI assistant authorization screen (the consent preview is fixed for the request),
  - the instance discovery document (already cached for 5 minutes),
  - screens with their own polling cadence (host metrics in genmonitor, platform update status) keep it.
- A failed background refresh keeps the data already on screen; only a failed first load shows the error, as today.
- On the board, the automatic refresh pauses while a move or reassignment the person just made is being saved, so the optimistic result never flickers back; it resumes when the save finishes.
- Drafts are never overwritten: an open task dialog, the board settings form, and the GitHub settings form keep what the person is typing while the data behind them refreshes.

Components touched: **web** only (`web/src/App.tsx`, `web/src/shared/`, and the screens that opt out or show load errors). No API changes: polling uses the existing read endpoints, which every component already authorizes.

## Capabilities

### New Capabilities

- `web-live-refresh`: automatic refresh of server data in the web app, its exceptions, and how it interacts with in-flight changes, drafts, and errors.

### Modified Capabilities

None — no capability has been archived into `openspec/specs/` yet.

## Non-goals

- Server push (Server-Sent Events or WebSockets); the developer chose polling.
- Picking the event bus technology (still an open decision in the constitution).
- Merging someone else's edits into an open draft, or warning about conflicting edits.
- Per-person settings for the interval, or a "live" indicator.
- Native desktop clients.

## Impact

- `web/src/shared/domain/`: the refresh interval and the pause rule, with tests.
- `web/src/App.tsx`: `QueryClient` defaults (`refetchInterval`, `refetchOnWindowFocus: true`; background polling off).
- `web/src/app/gentasks/BoardPage.tsx` and `components/useBoardMutations.ts`: pause while a board move is saving.
- `AcceptInvitePage`, `useAuthorizationPreview`, `useInstanceName`: explicit opt-out.
- Screens that render a query error: show it only for a failed first load (`isLoadingError`).
- Load: one read request per visible screen query every 10 seconds per person; every endpoint polled is a small indexed read.
