### Phased delivery

**Exception to rules 42, 44, and 46, approved by the developer:** this change ships together with `gentasks-task-description-markdown`, `gentasks-task-pull-requests`, and `web-live-refresh` on one branch, `feat/task-pull-requests-markdown-live-refresh`, in **one PR**. The phases still set the implementation and review order, each one starts with its tests and ends with lint and tests green, and the PR is opened only after every phase of the three changes and the developer's manual verification (rule 41).

| Phase | Contents | Depends on | Parallel with |
|-------|----------|------------|---------------|
| 1 | gentasks foundation: migration `00005`, snapshot and link models, repositories | — | 4 |
| 2 | gentasks webhook: decode state/draft/author/`updated_at`, `converted_to_draft`, snapshot + link/unlink in the sync service | 1 | 3, 4 |
| 3 | gentasks board view: `pullRequests` on each task, URL from `GITHUB_WEB_URL` | 1 | 2, 4 |
| 4 | web logic: types, `domain/pullRequests.ts`, `haveSameContent`, MSW fixtures | — (contract in specs) | 1–3 |
| 5 | web UI: state icon, list, card chip menu, dialog section, i18n | 3, 4 | — |
| 6 | full gates, manual verification, single PR | 1–5 | — |

Go phases follow rule 47's order (test → migration → model → repository → authorization → request type → service → handler → response type).

## Phase 1 — gentasks foundation

- [x] 1.1 Write repository tests in `modules/gentasks/tests/githubsync/`: migrating a database with `00004` link rows yields one snapshot per distinct pull request (token taken from its links, null title/state/author) and keeps `last_moved_event`, `moved_at`, and `merge_moved_at`; snapshot upsert stores the fields and reports a change, ignores an older `github_updated_at`, and accepts an equal one; linking a task twice keeps one active link; unlinking by keys soft-deletes only links whose task key is not cited and keeps archived cited tasks linked; relinking after unlink creates a new active link; `ListMergeLockedTaskIDs` still reports a task whose merged link was unlinked; `ListActiveForTasks` returns active links with snapshot fields for many tasks in one query and nothing for unlinked ones
- [x] 1.2 Write `modules/gentasks/migrations/00005_task_pull_request_snapshots.sql` (design §2) with a working `Down`
- [x] 1.3 Add `PullRequestState` (typed, rule 15), `GithubPullRequest`, `PullRequestAuthor`, and `LinkedPullRequest` models; update `TaskPullRequest` for the new columns
- [x] 1.4 Implement `GithubPullRequestRepository` (find by repository and number, conditional upsert) and extend `TaskPullRequestRepository` (link, unlink by keys, `RecordMove` by `pull_request_id`, `ListActiveForTasks`)
- [x] 1.5 Run `make lint` and the gentasks tests

## Phase 2 — gentasks webhook

- [x] 2.1 Add recorded fixtures for `converted_to_draft`, `closed` without merge, an `edited` that drops a key, and an older redelivery; make sure every `pull_request` fixture carries `state`, `draft`, `updated_at`, and `user`
- [x] 2.2 Write tests: state derivation (open, draft, merged, closed); decoding of the new fields; a delivery that cites a task links it without moving it when the task is already further right; `converted_to_draft` and unmerged `closed` update the snapshot and move nothing; an edit dropping `GEN-7` unlinks it and keeps `GEN-8`; an older delivery changes nothing; an uncited unknown pull request stores nothing; `labeled` changes nothing; all existing movement and lock tests still pass unchanged
- [x] 2.3 Extend `PullRequestEventRequest`, `PullRequestEvent`, and `PullRequestAction` (design §3)
- [x] 2.4 Update `PullRequestSyncService.SyncPullRequest` to the order in design §4
- [x] 2.5 Run `make lint` and the gentasks tests

## Phase 3 — gentasks board view

- [x] 3.1 Write tests in `modules/gentasks/tests/boardview/` and `tests/app/`: board view returns `pullRequests` with snapshot fields and `url` built from `GITHUB_WEB_URL`; `[]` for tasks without links; unlinked pull requests absent; migrated links have `null` title, state, and author and a URL; still returned with GitHub disconnected; `GET /tasks/{token}` and the task lists keep their shape
- [x] 3.2 Add the pull request URL function in `githubsync` (design §6)
- [x] 3.3 Load links in `BoardViewService.ReadBoard` with `ListActiveForTasks` and wire `GITHUB_WEB_URL` at the composition root
- [x] 3.4 Add `BoardTaskResponse` and `TaskPullRequestResponse` and use them in `NewBoardViewResponse`
- [x] 3.5 Run `make lint` and the gentasks tests

## Phase 4 — web logic

- [x] 4.1 Add pull request fixtures to `web/src/test/gentasksFixtures.ts` and include them in the board MSW handler
- [x] 4.2 Write tests: `sortPullRequests` follows the spec order and number descending; `pullRequestReference` and `pullRequestTitle` fall back to `owner/repo#number`; `pullRequestStateKey` maps `null` to `unknown`; `haveSameContent` is false when only `pullRequests` differ; `readBoard` returns `pullRequests` on each task
- [x] 4.3 Add the types and `domain/pullRequests.ts`; update `haveSameContent` (design §8)
- [x] 4.4 Run `make lint` and the web tests

## Phase 5 — web UI

- [x] 5.1 Add English and pt-BR copy: chip count with plurals, menu label, dialog section title, state names for icons' accessible labels
- [x] 5.2 Implement `PullRequestStateIcon` and `PullRequestList` (design §7–8)
- [x] 5.3 Implement `TaskPullRequestsMenu` on the card, outside the open button, keeping pointer, click, and key events from the card and drag sensor
- [x] 5.4 Add the "Pull requests" section to the task dialog's properties panel
- [x] 5.5 Run `make lint` and the web tests

## Phase 6 — gates and PR

- [x] 6.1 Run `make lint`, `make test`, and `make build`
- [ ] 6.2 Manual verification with the developer (rule 41): open a PR citing a task → chip "1 PR" with title and open icon; request review, convert to draft, close, reopen, and merge → icon and state follow; edit the title to drop the key → the chip disappears; add it back → it returns; clicking an item opens GitHub in a new tab without opening the dialog or dragging the card; keyboard navigation of the menu; dialog section; a link from before the change shows `owner/repo#number` and a working link; dark and light themes
- [x] 6.3 Read the whole diff (rule 80) and open the PR after sign-off
