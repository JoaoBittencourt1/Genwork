## Purpose

Lets a member ask a reviewer bot to review the pull requests linked to a task, with the outcome shown in the task conversation and on GitHub.

## ADDED Requirements

### Requirement: Request a review
genagents SHALL accept a review request for a task naming an enabled `reviewer` bot and the requesting member, and SHALL queue a review run. When no bot is named and exactly one reviewer bot is enabled, that bot SHALL be used; when several are enabled and none is named, the request SHALL be rejected with 400. A task without open or draft linked pull requests SHALL be rejected with 409. A task SHALL have at most one active review run.

#### Scenario: Single reviewer bot
- **WHEN** a member requests a bot review of `GEN-42`, which has one open pull request, and "Review Bot" is the only reviewer
- **THEN** a review run is queued with "Review Bot" and appears in the task conversation

#### Scenario: No pull request
- **WHEN** a bot review is requested for a task with no linked open or draft pull request
- **THEN** the request is rejected with 409 and the reason is shown

#### Scenario: Review already running
- **WHEN** a second review is requested while one is active for the same task
- **THEN** the request is rejected with 409

### Requirement: Review scope
A review run SHALL review each open or draft pull request linked to the task: its diff against its base branch and the task's title and description as the acceptance context. It SHALL check correctness, whether the change does what the task asks, tests, and the repository's own conventions files when present (for example a constitution or contributor guide).

#### Scenario: Two linked pull requests
- **WHEN** a task has two open linked pull requests
- **THEN** the review covers both and reports per pull request

### Requirement: Review outcome
A review run SHALL post on each reviewed pull request a GitHub review with a summary and line comments, using the event `COMMENT` (it never approves or requests changes in this delivery), and SHALL post in the task conversation a summary per pull request with the count of findings and a link to the GitHub review.

#### Scenario: Findings posted
- **WHEN** "Review Bot" finds two issues in `acme/api#12`
- **THEN** `acme/api#12` has a review with two line comments, and the task conversation shows "2 findings" with a link

#### Scenario: GitHub refuses the review
- **WHEN** posting the GitHub review fails
- **THEN** the full review text is posted in the task conversation and the run fails with reason `github_post_failed`
