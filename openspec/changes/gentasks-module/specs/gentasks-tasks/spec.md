## Purpose

Defines task cards on a board — their fields, urgency, assignee, and labels — and how they move between columns, get archived and restored, and get deleted.

## ADDED Requirements

### Requirement: Urgency levels
Task urgency SHALL be one of five values, ordered from lowest to highest: `none`, `low`, `medium`, `high`, `urgent`. New tasks default to `none`. Any other value is rejected with `400`.

#### Scenario: Default urgency
- **WHEN** a task is created without urgency
- **THEN** its urgency is `none`

#### Scenario: Unknown urgency
- **WHEN** urgency `critical` is sent
- **THEN** gentasks responds `400` with a validation error on `urgency`

### Requirement: Create a task
`POST /boards/{token}/tasks` (permission `gentasks.task.write`) SHALL create a task with a required title (1 to 200 characters after trimming) and these optional fields: `description` (up to 10,000 characters), `columnToken` (defaults to the board's first column), `assigneeToken`, `labelTokens`, and `urgency`. The task is placed last in its column. The response includes `createdBy`, the creator's user token.

#### Scenario: Minimal task
- **WHEN** a user creates a task with only a title
- **THEN** gentasks responds `201` and the task is last in the board's first column, with urgency `none`, no assignee, and no labels

#### Scenario: Column from another board
- **WHEN** `columnToken` belongs to a different board
- **THEN** gentasks responds `400` with a validation error on `columnToken`

#### Scenario: Label from another board
- **WHEN** a label token belongs to a different board or is deleted
- **THEN** gentasks responds `400` with a validation error on `labelTokens`

### Requirement: Assignee is a core user
An assignee SHALL be referenced only by core's user token (rule 14). On create and update, gentasks MUST confirm that the token belongs to a user who is not deleted through core's user directory, calling it with the caller's access token. Gentasks never stores user names or e-mails.

#### Scenario: Valid assignee
- **WHEN** `assigneeToken` belongs to an existing user
- **THEN** the task stores that token as its assignee

#### Scenario: Unknown assignee
- **WHEN** core answers `404` for the token
- **THEN** gentasks responds `400` with a validation error on `assigneeToken`

#### Scenario: Core unavailable
- **WHEN** core cannot be reached while validating an assignee
- **THEN** gentasks responds `503` and does not save the task

### Requirement: Update a task partially
`PATCH /tasks/{token}` (`gentasks.task.write`) SHALL follow rule 17 for `title`, `description`, `assigneeToken`, `urgency`, and `labelTokens`. An omitted field stays unchanged. `null` clears `description` and `assigneeToken`. `null` on `title`, `urgency`, or `labelTokens` responds `400`. `labelTokens` replaces the task's whole label set, and `[]` removes all labels.

#### Scenario: Single-field update
- **WHEN** the body is `{ "urgency": "high" }`
- **THEN** only urgency changes, and title, description, assignee, labels, column, and position are unchanged

#### Scenario: Unassign
- **WHEN** the body is `{ "assigneeToken": null }`
- **THEN** the task has no assignee

#### Scenario: Null title
- **WHEN** the body is `{ "title": null }`
- **THEN** gentasks responds `400`

### Requirement: Move a task
`POST /tasks/{token}/move` with `{ "columnToken": ..., "position": n }` (`gentasks.task.write`) SHALL place an active task at zero-based index `n` among the active tasks of the target column, which may be the task's current column. Positions in the source and target columns stay contiguous. An index past the end places the task last. Concurrent moves on the same board are serialized, so positions never collide.

#### Scenario: Move to another column
- **WHEN** a task is moved from To-do to Done at position 0
- **THEN** it is first in Done, and the remaining To-do tasks close the gap

#### Scenario: Reorder within a column
- **WHEN** the third task of a column is moved to position 0
- **THEN** it is first and the others shift down by one

#### Scenario: Target column on another board
- **WHEN** the target column belongs to a different board
- **THEN** gentasks responds `400`

#### Scenario: Move an archived task
- **WHEN** the task is archived
- **THEN** gentasks responds `409` with code `task_archived`

### Requirement: Archive and restore
`POST /tasks/{token}/archive` (`gentasks.task.write`) SHALL hide a task from the board and close the gap in its column. `POST /tasks/{token}/unarchive` SHALL restore it as the last task of its column. `GET /boards/{token}/tasks?archived=true` (`gentasks.board.read`) SHALL list the archived tasks, most recently archived first.

#### Scenario: Archive
- **WHEN** a task is archived
- **THEN** it no longer appears in `GET /boards/{token}` and does appear in the archived list

#### Scenario: Restore
- **WHEN** an archived task is restored
- **THEN** it appears last in its original column

#### Scenario: Archive twice
- **WHEN** an already archived task is archived again
- **THEN** gentasks responds `204` and nothing changes

### Requirement: Delete a task
`DELETE /tasks/{token}` (`gentasks.task.delete`) SHALL soft-delete an active or archived task (rule 22) and close the gap in its column if it was active. A deleted task is never returned again, and any request to it responds `404` with code `task_not_found`.

#### Scenario: Delete
- **WHEN** a user deletes a task
- **THEN** gentasks responds `204`, and the task appears neither on the board nor in the archived list

### Requirement: Read a task
`GET /tasks/{token}` (`gentasks.board.read`) SHALL return the task with its column token, position, title, description, assignee token, urgency, labels (token, name, color), creator token, `archivedAt`, `createdAt`, and `updatedAt`.

#### Scenario: Task on a deleted board
- **WHEN** the task's board has been deleted
- **THEN** gentasks responds `404` with code `task_not_found`
