## Purpose

Lets gentasks tasks be assigned to bots and reviewed by bots, integrating with genagents as an optional module that degrades gracefully when absent.

## ADDED Requirements

### Requirement: Bot members as assignees
gentasks SHALL accept an active bot member as a task assignee exactly like a human member, and the board view and task reads SHALL return the assignee's kind. The assignee picker SHALL group bot members under "Bots" and SHALL show only `dev` bots there when genagents is enabled, using the genagents bot list; when genagents is disabled, bot members SHALL NOT be offered.

#### Scenario: Assign a bot
- **WHEN** a member assigns `GEN-42` to "Dev Bot"
- **THEN** the task's assignee is the bot member and the card shows a bot avatar

#### Scenario: genagents disabled
- **WHEN** genagents is disabled
- **THEN** the picker offers only human members and existing bot assignees stay visible

### Requirement: Assignment notification
When a task's assignee changes to or from a bot member, gentasks SHALL notify genagents through its versioned API, on behalf of the acting member, with the task token and the old and new assignee tokens, after the change is committed. When genagents is not enabled the notification SHALL be skipped. When the notification fails, the assignment SHALL still succeed; genagents SHALL pick the change up through reconciliation: at least every 60 seconds it reads each enabled dev bot's assigned tasks from gentasks as that bot, starting runs for new assignments and cancelling runs for tasks no longer assigned. genagents SHALL treat a notification only as a hint and SHALL confirm the assignment by reading the task before starting a run.

#### Scenario: Notification fails
- **WHEN** genagents is unreachable at assignment time and back two minutes later
- **THEN** the assignment succeeded immediately and the run is queued within 60 seconds of genagents being back

#### Scenario: Forged notification
- **WHEN** a caller notifies genagents that a task is assigned to "Dev Bot" but the task is assigned to someone else
- **THEN** no run starts

### Requirement: Board repository
A board's GitHub settings SHALL gain an optional `repository` (`owner/name`), set through the existing settings `PATCH` (rule 17) by callers holding `gentasks.board.manage`, and validated against the `owner/name` format. It is the repository bots work in for that board's tasks.

#### Scenario: Set the repository
- **WHEN** an admin patches a board's GitHub settings with `repository: "acme/api"`
- **THEN** the setting is stored and every other GitHub setting of the board is unchanged

#### Scenario: Invalid repository
- **WHEN** the repository is `acme`
- **THEN** the request is rejected with 400

### Requirement: Task context for bots
gentasks SHALL expose, to callers holding `gentasks.board.read`, a task context read with the task's key, title, description, labels, board, linked pull requests, and the board's repository, so genagents builds run prompts through the public API (rule 54).

#### Scenario: Bot reads its task
- **WHEN** a bot member reads the context of its assigned task
- **THEN** the response contains the fields above and the repository `owner/name`

### Requirement: Bot review request
gentasks SHALL expose `POST /api/gentasks/v1/tasks/{token}/bot-reviews`, requiring `gentasks.task.write`, that forwards the request (optional reviewer bot token) to genagents and returns its result, including its 400 and 409 errors. When genagents is not enabled it SHALL answer 404.

#### Scenario: Request through gentasks
- **WHEN** a member presses "Bot review" on `GEN-42`
- **THEN** gentasks forwards the request and the review appears in the task conversation

### Requirement: Task dialog additions
When genagents is enabled, the task dialog SHALL show a "Bot review" button (with a reviewer picker when there is more than one reviewer bot), disabled with an explanation when the task has no open or draft linked pull request, and a "Bot conversation" panel showing the task conversation, polling every 2 seconds while a run is active and every 10 seconds otherwise, with a message box and a "Stop" button for active runs. When genagents is disabled, neither SHALL be shown.

#### Scenario: Watching a bot work
- **WHEN** a member opens `GEN-42` while "Dev Bot" works on it
- **THEN** the panel shows the bot's status updates as they arrive and lets the member reply

#### Scenario: No pull request yet
- **WHEN** a task has no linked open or draft pull request
- **THEN** "Bot review" is disabled with the reason
