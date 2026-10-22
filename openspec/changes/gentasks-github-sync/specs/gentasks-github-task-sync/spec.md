## Purpose

Moves gentasks tasks automatically when GitHub reports that a pull request citing them was opened, had a review requested, or was merged into a final branch.

## ADDED Requirements

### Requirement: Webhook endpoint authenticated by signature
`POST /api/gentasks/v1/github/webhook` SHALL be reachable without an access token and SHALL accept a delivery only when its `X-Hub-Signature-256` header is the HMAC-SHA256 of the raw body under the stored webhook secret, compared in constant time. That signature is the endpoint's authorization rule (rule 18). Bodies larger than 5 MiB SHALL be rejected without being read further.

#### Scenario: Valid signature
- **WHEN** GitHub delivers a `pull_request` event signed with the stored secret
- **THEN** gentasks processes it and responds `204`

#### Scenario: Missing or wrong signature
- **WHEN** the signature header is missing, malformed, or does not match
- **THEN** gentasks responds `401` and moves nothing

#### Scenario: Not connected
- **WHEN** no App is stored, or its credentials are unreadable
- **THEN** gentasks responds `401` and moves nothing

#### Scenario: Oversized body
- **WHEN** the body exceeds 5 MiB
- **THEN** gentasks responds `413`

### Requirement: Events handled
gentasks SHALL act only on `pull_request` events with the actions `opened`, `reopened`, `edited`, `ready_for_review`, `review_requested`, and `closed`. Every other event (including `ping` and `installation`) and every other action SHALL be acknowledged with `204` and ignored.

#### Scenario: Ping
- **WHEN** GitHub sends the `ping` event after the App is created
- **THEN** gentasks responds `204`

#### Scenario: Unhandled action
- **WHEN** a `pull_request` event has action `labeled`
- **THEN** gentasks responds `204` and moves nothing

### Requirement: Task keys in a pull request
A pull request SHALL cite every task whose key — the board's task prefix, a hyphen, and the task number — appears in the PR title or head branch name, matched case-insensitively at word boundaries (e.g. `GEN-42` in the title, `feat/gen-42-login` as the branch). Keys that match no active task, or that match an archived or deleted task, SHALL be ignored. The PR body SHALL NOT be searched.

#### Scenario: Key in the branch name
- **WHEN** a PR's head branch is `feat/gen-42-login` and board `GEN` has task 42
- **THEN** task `GEN-42` is cited

#### Scenario: Several keys
- **WHEN** the title is `GEN-1 and OPS-7: share login`
- **THEN** both `GEN-1` and `OPS-7` are cited, each evaluated against its own board's settings

#### Scenario: Key only in the body
- **WHEN** a task key appears only in the PR body
- **THEN** no task is cited

#### Scenario: Unknown key
- **WHEN** the title cites `GEN-999` and board `GEN` has no task 999
- **THEN** the key is ignored and the delivery still responds `204`

### Requirement: Movement rules
For each cited task, gentasks SHALL pick a target column from the task's board settings: `opened`, `reopened`, `edited`, and `ready_for_review` use the opened column; `review_requested` uses the review requested column; `closed` with `merged: true` and a base branch listed in the board's final branches uses the merged column. A `closed` event without a merge, a merge into a branch not listed, or an event whose column is not configured SHALL move nothing. A task SHALL be moved only when the target column is further right on the board than its current column, and is placed last in the target column. A task the automation moved to the merged column SHALL never be moved by the automation again. Manual moves SHALL remain unrestricted.

#### Scenario: PR opened
- **WHEN** a PR citing a task in `Todo` is opened and the board's opened column is `In progress`, to the right of `Todo`
- **THEN** the task moves to the end of `In progress`

#### Scenario: Review requested
- **WHEN** a review is requested on a PR citing a task in `In progress` and the review requested column is `In review`
- **THEN** the task moves to `In review`

#### Scenario: Merged into a final branch
- **WHEN** a PR citing a task is merged into `main` and `main` is a final branch of the board
- **THEN** the task moves to the merged column and the automation will not move it again

#### Scenario: Merged into another branch
- **WHEN** the PR is merged into `develop` and `develop` is not a final branch
- **THEN** the task does not move

#### Scenario: Closed without merging
- **WHEN** the PR is closed with `merged: false`
- **THEN** the task does not move

#### Scenario: Never backwards
- **WHEN** a task is already in `In review` and the PR is edited
- **THEN** the task stays in `In review`

#### Scenario: Locked after merge
- **WHEN** the automation moved a task to `Done` on merge, someone dragged it back to `Todo`, and a new PR citing it is opened
- **THEN** the automation does not move it

#### Scenario: Event not configured
- **WHEN** the board's review requested column is `null`
- **THEN** a review request does not move the task

#### Scenario: Redelivered event
- **WHEN** GitHub delivers the same event twice
- **THEN** the second delivery changes nothing

### Requirement: Movements attributed to Genwork
Moves made by the automation SHALL not be attributed to any user. gentasks SHALL record, for each task and pull request (repository and number), the last event that moved the task and whether the move came from a merge.

#### Scenario: Move recorded
- **WHEN** the automation moves a task because of PR `acme/api#12`
- **THEN** gentasks records the link between the task and `acme/api#12` with the event that caused the move
