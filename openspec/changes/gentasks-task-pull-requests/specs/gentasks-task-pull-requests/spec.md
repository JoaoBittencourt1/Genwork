## ADDED Requirements

### Requirement: Pull request snapshot
For every `pull_request` delivery with a handled action that cites at least one active task, or that concerns a pull request gentasks already knows, gentasks SHALL store one snapshot per pull request (repository and number): title, state, author login, author avatar URL, and GitHub's `pull_request.updated_at`. The state SHALL be `merged` when the pull request is merged, `closed` when it is closed without merging, `draft` when it is open as a draft, and `open` otherwise. A delivery whose `updated_at` is older than the stored one SHALL NOT change the snapshot and SHALL NOT link or unlink tasks; it SHALL still apply the movement rules, which only move tasks forward, and a task it moves is linked by that move.

#### Scenario: Opened
- **WHEN** GitHub delivers `opened` for `acme/api#12` titled "GEN-7 add export" by `octocat`
- **THEN** the snapshot of `acme/api#12` has that title, state `open`, and author `octocat` with the payload's avatar URL

#### Scenario: Converted to draft
- **WHEN** GitHub delivers `converted_to_draft` for a known pull request
- **THEN** its snapshot state becomes `draft` and no task moves

#### Scenario: Closed without merging
- **WHEN** GitHub delivers `closed` with `merged: false` for a known pull request
- **THEN** its snapshot state becomes `closed` and no task moves

#### Scenario: Merged
- **WHEN** GitHub delivers `closed` with `merged: true`
- **THEN** its snapshot state becomes `merged`, and movement follows the existing merge rules

#### Scenario: Title edited
- **WHEN** GitHub delivers `edited` with a new title
- **THEN** the snapshot title is the new title

#### Scenario: Out-of-order delivery
- **WHEN** a delivery with `updated_at` 10:00 arrives after one with `updated_at` 10:05 for the same pull request
- **THEN** the snapshot and links keep the 10:05 values

#### Scenario: Late delivery still moves
- **WHEN** an `edited` delivery from 12:05 citing `GEN-2` arrives after a `reopened` delivery from 12:30 citing only `GEN-1`, and `GEN-2` is in a column before "PR opened"
- **THEN** `GEN-2` moves to the "PR opened" column and is linked by the move, while the snapshot keeps the 12:30 title

#### Scenario: Uncited and unknown pull request
- **WHEN** a delivery cites no task and gentasks has no snapshot of that pull request
- **THEN** nothing is stored

### Requirement: Tasks are linked to every pull request that cites them
On every delivery that changes the snapshot, gentasks SHALL link each active task the pull request cites (title or head branch, as in the movement rules) to the pull request, whether or not the delivery moves the task, and SHALL unlink (soft delete) every linked task whose key the pull request no longer cites. Citing an unlinked task again SHALL link it again. Archived tasks keep their existing links but are not newly linked. Unlinking SHALL NOT release a task that a merge already locked.

#### Scenario: Linked without moving
- **WHEN** a pull request citing `GEN-7` is opened while `GEN-7` is already in a column to the right of the "PR opened" column
- **THEN** `GEN-7` does not move and is linked to the pull request

#### Scenario: Key removed from the title
- **WHEN** the title of a pull request citing `GEN-7` and `GEN-8` is edited to cite only `GEN-8`, and the head branch does not cite `GEN-7`
- **THEN** `GEN-7` is unlinked and `GEN-8` stays linked

#### Scenario: Key added back
- **WHEN** the title is edited again to cite `GEN-7`
- **THEN** `GEN-7` is linked again

#### Scenario: Several pull requests
- **WHEN** two pull requests in different repositories cite `GEN-7`
- **THEN** `GEN-7` is linked to both

#### Scenario: Merge lock survives unlinking
- **WHEN** a merged pull request moved `GEN-7` and is later edited to stop citing it
- **THEN** `GEN-7` is unlinked and automation still never moves it again

### Requirement: Events handled
gentasks SHALL act on `pull_request` events with the actions `opened`, `reopened`, `edited`, `ready_for_review`, `converted_to_draft`, `review_requested`, and `closed`. Every other event and action SHALL be acknowledged with `204` and ignored, as before. `converted_to_draft` and `closed` without merge SHALL update snapshots and links only.

#### Scenario: Unhandled action
- **WHEN** a `pull_request` event has action `labeled`
- **THEN** gentasks responds `204` and changes neither snapshots, links, nor tasks

### Requirement: Existing links keep working
Links recorded before this change SHALL be kept and shown with their repository and number, an unknown title, state, and author, and a working URL, until a delivery for that pull request fills in its snapshot. Movement history (`last_moved_event`, `moved_at`, `merge_moved_at`) SHALL be preserved.

#### Scenario: Link from before the change
- **WHEN** the board view is read after the migration for a task an earlier merge of `acme/api#3` moved
- **THEN** the task lists `acme/api#3` with `title`, `state`, and `author` set to `null` and a URL to the pull request, and the task stays merge-locked

### Requirement: Linked pull requests in the board view
`GET /api/gentasks/v1/boards/{token}` SHALL return, on every task, a `pullRequests` array of its active links: `token`, `repository`, `number`, `title` (nullable), `state` (nullable), `author` (`login` and `avatarUrl`, nullable), and `url` (`<GITHUB_WEB_URL>/<repository>/pull/<number>`), loaded for all the board's tasks in a fixed number of queries (rule 21). A task without links SHALL return an empty array. The endpoint's authorization (`gentasks.board.read`) is unchanged, and the response SHALL be returned whether or not GitHub is currently connected.

#### Scenario: Task with pull requests
- **WHEN** a client reads a board whose task `GEN-7` is linked to `acme/api#12` (open) and `acme/web#4` (merged)
- **THEN** `GEN-7`'s `pullRequests` contains both with their snapshot fields and URLs

#### Scenario: Task without pull requests
- **WHEN** a task has no active link
- **THEN** its `pullRequests` is `[]`

#### Scenario: Unlinked pull request
- **WHEN** a pull request was unlinked from a task
- **THEN** it does not appear in that task's `pullRequests`

#### Scenario: GitHub disconnected
- **WHEN** an admin disconnected GitHub after tasks were linked
- **THEN** the board view still returns the linked pull requests

#### Scenario: Fixed query count
- **WHEN** a board has 200 tasks with pull requests
- **THEN** the pull requests are read with one query
