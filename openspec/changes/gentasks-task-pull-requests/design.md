## Context

`gentasks-github-sync` added `internal/githubsync`: a signed webhook decodes `pull_request` deliveries into `PullRequestEvent` (action, repository, number, title, head and base refs, merged), extracts task keys from the title and head branch, and moves cited tasks forward. `task_pull_requests` keeps one row per task and pull request, but only when the pull request **moved** the task, and only stores the move (`last_moved_event`, `moved_at`, `merge_moved_at`) — no title, state, or author. Its non-goals deferred showing linked pull requests; this change delivers that.

The board view (`internal/boardview`) assembles columns, tasks, and labels in a fixed number of queries; the web board and task dialog read everything from it.

## Goals / Non-Goals

**Goals:**
- Know every pull request that cites a task, with enough data to render it like a code host does (state, title, reference, author) and link to it.
- Keep the data current from webhook deliveries only, robust to redelivery and out-of-order delivery.
- Keep the movement rules and the merge lock exactly as they are.

**Non-Goals:**
- GitHub API calls, checks or reviews, manual linking, search (see proposal).

## Decisions

### 1. One snapshot per pull request, links reference it
New table `github_pull_requests` holds the pull request facts once (rule 7); `task_pull_requests` becomes the link between a task and a pull request plus its move history. Storing title/state/author on each link would duplicate them for every task a pull request cites and let them diverge.

Alternative considered: adding the columns to `task_pull_requests` — simpler migration, but a duplicated fact per cited task.

### 2. Data model — migration `00005_task_pull_request_snapshots.sql`
- `github_pull_requests`: `id`, `token`, `repository` (`owner/name`), `number int CHECK > 0`, `title text` nullable, `state text` nullable `CHECK (state IN ('open','draft','merged','closed'))`, `author_login text` nullable, `author_avatar_url text` nullable, `github_updated_at timestamptz` nullable, `created_at`, `updated_at`; `UNIQUE (repository, number)`. Nullable fields exist only for rows migrated from before the change. Pull requests are not removable domain records — GitHub never deletes them in a way gentasks hears about — so no `deleted_at`.
- `task_pull_requests`:
  - add `pull_request_id bigint REFERENCES github_pull_requests (id)`; insert one `github_pull_requests` row per distinct existing `(repository, pull_request_number)` with only `token`, `repository`, `number`, and timestamps; set `pull_request_id`; make it `NOT NULL`;
  - drop `repository` and `pull_request_number` (now owned by the snapshot, rule 7) and the old unique constraint;
  - `last_moved_event` and `moved_at` become nullable (a link that never moved its task), with a `CHECK` that they are both null or both set;
  - add `deleted_at timestamptz` and replace the unique constraint with `CREATE UNIQUE INDEX ... (task_id, pull_request_id) WHERE deleted_at IS NULL` (rule 22).
- The `Down` reverses it (re-adds the columns from the join, deletes link rows that never moved a task, and drops the new table).

This is a new migration; `00004` is not edited (rule 20). New rows get tokens from `platform.NewPublicToken`. The migrated snapshots cannot: SQL in the module has no 128-bit random generator of its own (`pgcrypto` belongs to core's schema, which the module role must not depend on, rule 53, and `gen_random_uuid()` carries only 122 random bits, below rule 14). Each migrated snapshot therefore takes the smallest token among its existing links — already a `NewPublicToken` value — and no API has ever exposed link tokens, so the value stays unique per table and opaque.

### 3. Decoding more of the delivery
`PullRequestEventRequest` additionally reads `pull_request.state`, `pull_request.draft`, `pull_request.updated_at`, `pull_request.user.login`, and `pull_request.user.avatar_url`; `PullRequestEvent` gains `State` (typed `PullRequestState`, rule 15), `UpdatedAt`, and `Author`. `PullRequestState` derives from merged/state/draft in a pure function. `ActionConvertedToDraft` is added; `Trigger()` keeps returning "no move" for it and for an unmerged close, so movement is unchanged.

### 4. Sync order inside the existing transaction
`SyncPullRequest` keeps its single transaction and board locks (design §7 of `gentasks-github-sync`) and becomes:

1. Return early for unhandled actions.
2. Resolve cited active tasks from the keys (existing query).
3. Upsert the snapshot only when `cited` is non-empty or the pull request is already stored; the upsert's `WHERE github_pull_requests.github_updated_at IS NULL OR EXCLUDED.github_updated_at >= github_pull_requests.github_updated_at` makes stale deliveries no-ops and reports whether the row changed. When it did not, skip step 4 but still run step 5: the existing movement rules never depended on delivery order (an existing test delivers `edited` after `reopened`), and since moves only go forward a late delivery cannot undo a newer one.
4. Link each cited task (`INSERT ... ON CONFLICT (task_id, pull_request_id) WHERE deleted_at IS NULL DO NOTHING`), then soft-delete this pull request's active links whose task key is not among the delivery's keys — computed in SQL by joining the link's task and board against the keys, so archived tasks that are still cited stay linked. Both are single statements (rule 21).
5. Apply the existing movement rules; `RecordMove` now updates the active link by `(task_id, pull_request_id)`.

`ListMergeLockedTaskIDs` keeps reading **all** links, including soft-deleted ones, so unlinking never releases a merge lock (spec "Merge lock survives unlinking").

Redelivery stays idempotent: the same `updated_at` rewrites the same snapshot, links already exist, and moves only go forward.

### 5. Board view reads links in one query
`TaskPullRequestRepository.ListActiveForTasks(taskIDs)` joins `task_pull_requests` (active) with `github_pull_requests` and returns `map[taskID][]LinkedPullRequest` (rule 21). `BoardViewService` calls it next to the task labels; `boardview` already depends on other features' repositories (labels), so this follows the same composition.

The board view's task response becomes `BoardTaskResponse` — the existing `tasks.TaskResponse` embedded plus `pullRequests` — so `GET /tasks/{token}`, the task list, and the assigned-tasks list keep their current shape. Adding a field is not a breaking change to `v1` (rule 61).

### 6. URL from configuration
The response's `url` is `<GITHUB_WEB_URL>/<repository>/pull/<number>`, built by a `githubsync` function from the already-required `GITHUB_WEB_URL` (rule 1). It works for rows migrated without a snapshot and follows the configured GitHub host, so no `html_url` is stored. `BoardViewService` (or its handler) receives the base URL at the composition root.

### 7. Author avatar
The avatar URL is stored as GitHub sends it and rendered as an `<img>` with `referrerPolicy="no-referrer"`, `loading="lazy"`, and the login as `alt`; when it is `null` or fails to load the item shows no avatar. The gateway sets no `Content-Security-Policy` today, so nothing blocks the image host.

### 8. Web
Inside `web/src/app/gentasks/` (rule 72):
- `domain/types.ts`: `PullRequestState`, `TaskPullRequest`, and `BoardTask = Task & { pullRequests: TaskPullRequest[] }` for the board view's columns; tasks from other endpoints stay `Task`.
- `domain/pullRequests.ts`: `sortPullRequests` (spec order), `pullRequestReference` (`owner/repo#number`), `pullRequestTitle` (title or reference), `pullRequestStateKey` (state or `unknown`, for icon and label lookups). Unit-tested (rule 74).
- `components/PullRequestStateIcon.tsx`: lucide `GitPullRequest` (open, `text-primary`), `GitPullRequestDraft` (draft, `text-muted-foreground`), `GitMerge` (merged, `text-primary`), `GitPullRequestClosed` (closed, `text-destructive`), `GitPullRequest` (unknown, `text-muted-foreground`) — theme variables only (rule 81).
- `components/PullRequestList.tsx`: the list items, links with `target="_blank"` and `rel="noopener noreferrer"`, shared by card and dialog.
- `components/TaskPullRequestsMenu.tsx`: the card chip, built on the existing `DropdownMenu` (rule 73, no new shadcn component) with `DropdownMenuItem asChild` anchors. It sits outside the card's open button, like `TaskAssigneeMenu`, and stops pointer, click, and key events from reaching the card and the drag sensor.
- `TaskDialog.tsx`: a "Pull requests" property listing `PullRequestList` when the edited task has links. `haveSameContent` in `domain/ordering.ts` must include `pullRequests`, otherwise memoized cards would not re-render when a delivery adds one.
- Copy in English and pt-BR (rule 36), with i18next plurals for the count.

## Risks / Trade-offs

- [Pull requests opened before the change, or that never received a delivery since, show only `owner/repo#number`] → Acceptable and documented; any later delivery (edit, review request, merge) fills them in. A backfill through the GitHub API is a non-goal.
- [Missed webhook deliveries leave a stale state (e.g. "open" for a merged pull request)] → Same exposure as the movement automation; GitHub's "Recent deliveries" redelivery fixes it.
- [Author avatars are loaded from GitHub's host, which sees the viewer's IP address] → `referrerPolicy="no-referrer"` hides the Genwork URL; self-hosted admins already connect GitHub knowingly. If a CSP is added later, it must allow the avatar host or the avatar silently disappears.
- [The migration rewrites `task_pull_requests`] → The table is small (one row per moved task); the migration runs in goose's transaction and has a `Down`.
- [A pull request citing many tasks] → Linking and unlinking are single statements over the cited keys, already bounded by the key regex.

## Migration Plan

1. Deploy the module: goose runs `00005`, migrating existing links to snapshots without titles.
2. Deploy the web bundle in the same release; until then the old bundle ignores the new `pullRequests` field.
3. Rollback: the `Down` restores `00004`'s shape and drops links that never moved a task (they did not exist before).

## Open Questions

None.
