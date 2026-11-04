### Phased delivery

**Exception to rules 42 and 44, approved by the developer:** this change ships together with `gentasks-task-description-markdown` and `gentasks-task-pull-requests` on one branch, `feat/task-pull-requests-markdown-live-refresh`, in **one PR**. The phases still set the implementation order, each one starts with its tests and ends with lint and tests green, and the PR is opened only after the developer's manual verification (rule 41).

| Phase | Contents | Depends on | Parallel with |
|-------|----------|------------|---------------|
| 1 | Refresh interval and pause rule in `shared/domain/`; `QueryClient` defaults; opt-outs | — | — |
| 2 | Board pause while saving; load-error-only display across screens; manual verification | 1 | — |

## Phase 1 — refresh defaults

- [x] 1.1 Write tests in `web/src/shared/domain/liveRefresh.test.ts`: `liveRefreshInterval({ savingChanges: false })` is `LIVE_REFRESH_INTERVAL_MS` (10 000) and `liveRefreshInterval({ savingChanges: true })` is `false`
- [x] 1.2 Implement `web/src/shared/domain/liveRefresh.ts`
- [x] 1.3 Set the `QueryClient` defaults in `App.tsx` (design §1)
- [x] 1.4 Opt out in `AcceptInvitePage`, `useAuthorizationPreview`, and `useInstanceName` (design §2)
- [x] 1.5 Run `make lint` and the web tests

## Phase 2 — board and errors

- [x] 2.1 Give the board mutations in `useBoardMutations.ts` a per-board mutation key and pause the board query in `BoardPage.tsx` while they run (design §3)
- [x] 2.2 Add `loadError` to `shared/domain/errors.ts` with tests, and switch every screen that shows a query error to it or to `isLoadingError` (design §4): `BoardPage`, `BoardSettingsPage`, `BoardsPage`, `GithubIntegrationPage`, `GithubSettingsSection`, `HomeAssignedTasksSection`, `HomeBoardsSection`, `ArchivedTasksDialog`, `HostPage`, `MemberTable`, `InviteTable`, and the capabilities provider if it shows one
- [x] 2.3 Run `make lint`, `make test`, and `make build`
- [ ] 2.4 Manual verification with the developer (rule 41): two browsers on one board see each other's moves, edits, new tasks, and archives within 10 s; a GitHub-driven move appears without reload; members and invites pages update; a hidden tab stops polling (network panel) and refreshes on return; stopping the API keeps the board on screen without error; a slow move does not flicker back; an open task dialog and the settings forms keep their drafts
