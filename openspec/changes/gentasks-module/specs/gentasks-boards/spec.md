## Purpose

Lets people create task boards and shape each board's workflow as an ordered set of columns, starting from Backlog, To-do, and Done.

## ADDED Requirements

### Requirement: Create a board with default columns
`POST /api/gentasks/v1/boards` (permission `gentasks.board.manage`) SHALL create a board with a name of 1 to 80 characters after trimming. In the same transaction it creates three columns in this order, marked with the presets `backlog`, `todo`, and `done` and with no custom name. The response returns the board with its columns.

#### Scenario: New board
- **WHEN** a user creates a board named "Platform"
- **THEN** gentasks responds `201` with the board and three columns, in order, with presets `backlog`, `todo`, `done` and `name: null`

#### Scenario: Invalid name
- **WHEN** the name is empty after trimming or longer than 80 characters
- **THEN** gentasks responds `400` with a validation error on `name`

### Requirement: Column display name comes from preset or custom name
A column SHALL have either a preset (`backlog`, `todo`, `done`) with no custom name, or a custom name of 1 to 60 characters. Clients show the localized preset label while the name is null. Renaming a preset column sets its custom name. Its preset stays stored, but clients stop showing the preset label once a name is set.

#### Scenario: Rename a default column
- **WHEN** a user renames the `todo` column to "Next up"
- **THEN** the column has `name: "Next up"` and `preset: "todo"`

#### Scenario: Clear a custom name of a preset column
- **WHEN** a `PATCH` on a preset column sends `name: null`
- **THEN** the name is cleared and clients show the preset label again

#### Scenario: Clear the name of a custom column
- **WHEN** a `PATCH` on a column without a preset sends `name: null`
- **THEN** gentasks responds `400`

### Requirement: List, read, rename, and delete boards
`GET /boards` (permission `gentasks.board.read`) SHALL list boards that are not deleted, ordered by name. `GET /boards/{token}` SHALL return the board with its columns in order, each column's active tasks in order, each task's labels, and the board's labels, using a fixed number of queries whatever the number of tasks (rule 21). `PATCH /boards/{token}` (manage) SHALL rename the board. `DELETE /boards/{token}` (manage) SHALL soft-delete the board, after which it and everything in it are unreachable.

#### Scenario: Read a board
- **WHEN** a user opens a board with 3 columns and 40 tasks
- **THEN** the response contains every column and active task in order, and archived and deleted tasks are left out

#### Scenario: Deleted board
- **WHEN** a board has been deleted
- **THEN** it is not listed, and any request under it responds `404` with code `board_not_found`

#### Scenario: Partial update
- **WHEN** `PATCH /boards/{token}` is sent with an empty body
- **THEN** every stored field is unchanged and the response is `200`

### Requirement: Add and reorder columns
`POST /boards/{token}/columns` (manage) SHALL append a column with a custom name. `POST /columns/{token}/move` with `{ "position": n }` (manage) SHALL place the column at zero-based index `n` among the board's columns and shift the others. An index past the end places it last.

#### Scenario: Add a column
- **WHEN** a user adds "Review" to a board with three columns
- **THEN** "Review" is the fourth column

#### Scenario: Move a column
- **WHEN** a user moves the fourth column to position 1
- **THEN** the order becomes backlog, Review, todo, done

#### Scenario: Negative position
- **WHEN** the position is negative
- **THEN** gentasks responds `400`

### Requirement: Delete a column
`DELETE /columns/{token}` (manage) SHALL soft-delete a column only when no task that is not deleted belongs to it, archived tasks included, and only when it is not the board's last column.

#### Scenario: Column with tasks
- **WHEN** a column holds an active or archived task
- **THEN** gentasks responds `409` with code `column_not_empty`

#### Scenario: Last column
- **WHEN** the column is the board's only remaining column
- **THEN** gentasks responds `409` with code `last_column`

#### Scenario: Empty column
- **WHEN** the column is empty and others exist
- **THEN** gentasks responds `204`, and the remaining columns keep their relative order with no gaps
