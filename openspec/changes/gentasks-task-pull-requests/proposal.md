## Why

The GitHub integration already moves a task when a pull request that cites it is opened, reviewed, or merged, but the task never shows which pull request did it. People have to search GitHub by hand to find the code behind a card, and cannot tell at a glance whether a task has a pull request at all.

## What Changes

- gentasks keeps a snapshot of every pull request that cites a task: repository, number, title, state (`open`, `draft`, `merged`, `closed`), and author (login and avatar URL), updated from each `pull_request` webhook delivery. Out-of-order deliveries never overwrite a newer snapshot (GitHub's `pull_request.updated_at` decides).
- A task is linked to every pull request that cites it, not only to the ones that moved it. Editing a pull request so it no longer cites a task unlinks it (soft delete); citing it again links it again.
- The webhook additionally handles `converted_to_draft` and records a pull request closed without merging as `closed`. Movement rules do not change.
- The board view (`GET /api/gentasks/v1/boards/{token}`) returns, for each task, its linked pull requests with a link to the pull request on GitHub, built from `GITHUB_WEB_URL`. Additive change to the `v1` contract (rule 61).
- Web:
  - Task card: a chip with the pull request icon and count ("1 PR", "4 PRs"); opening it lists the pull requests (state icon, title, `owner/repo#number`, author avatar). Choosing one opens it on GitHub in a new tab.
  - Task dialog: a "Pull requests" section in the properties panel with the same list.
- Links recorded before this change (moves only) keep working: they show `owner/repo#number` and a working link until the pull request's next delivery fills in its title, state, and author.

Components touched: **gentasks module** (migration, `internal/githubsync`, `internal/boardview`) and **web** (`web/src/app/gentasks/`). Core, the CLI, and the module's `module.yaml` are not touched (no new permission, event, or route). The change spans the module and the web app because the data only exists in the module and is only useful on screen (rule 63); phases are split per component in tasks.md.

## Capabilities

### New Capabilities

- `gentasks-task-pull-requests`: the pull request snapshot, linking and unlinking tasks from webhook deliveries, and the linked pull requests in the board view response.
- `gentasks-task-pull-requests-web-ui`: the pull request chip on task cards and the pull request section in the task dialog.

### Modified Capabilities

None in `openspec/specs/` — the GitHub sync capabilities from `gentasks-github-sync` are not archived yet. This change extends its `gentasks-github-task-sync` behavior (events handled, links recorded); its movement rules are unchanged.

## Non-goals

- Fetching pull requests from the GitHub API (backfilling titles, polling): data comes only from webhook deliveries.
- Showing checks, reviewers, or review status, or the pull request body.
- Linking pull requests by hand, or unlinking from the UI.
- Searching inside the pull request list (the list is short).
- Pull requests in the assigned-tasks list on the home page or in the archived tasks dialog.
- GitHub issues, commits, or branches without a pull request.

## Impact

- `modules/gentasks/migrations/00005_task_pull_request_snapshots.sql`: new `github_pull_requests` table; `task_pull_requests` references it, gains `deleted_at`, and its move columns become nullable (existing rows are migrated).
- `modules/gentasks/internal/githubsync/`: event request and model gain state, draft, author, and `updated_at`; the sync service records snapshots and links; new repository for snapshots; batch read of links for many tasks (rule 21).
- `modules/gentasks/internal/boardview/`: board view gains each task's pull requests; the handler needs `GITHUB_WEB_URL` (already required config).
- Recorded webhook fixtures: new `converted_to_draft` and closed-without-merge payloads with `user` and `updated_at`.
- `web/src/app/gentasks/`: types, `domain/pullRequests.ts` with tests, MSW fixtures, card chip, dialog section, English and pt-BR copy.
- No new Go or npm dependencies. The browser loads author avatars from GitHub's avatar host.
