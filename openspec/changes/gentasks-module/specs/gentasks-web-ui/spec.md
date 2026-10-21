## Purpose

Defines how people use gentasks in the web app: browsing boards, working the board with drag and drop, and editing tasks, columns, and labels.

## ADDED Requirements

### Requirement: Boards list
The `/tasks` page SHALL list boards and, for users with `gentasks.board.manage`, offer creating, renaming, and deleting boards, with a confirmation before deletion. Selecting a board opens `/tasks/$boardToken`.

#### Scenario: Read-only user
- **WHEN** a user has `gentasks.board.read` but not `gentasks.board.manage`
- **THEN** the page lists boards without create, rename, or delete actions

### Requirement: Board view
The board page SHALL show columns left to right in order, with each column's active tasks top to bottom. Every card shows its title, urgency, label chips, and the assignee's name, which is resolved from the core user directory. A preset column with no custom name shows the localized preset label.

#### Scenario: Preset label in pt-BR
- **WHEN** the UI language is pt-BR and a `todo` column has no custom name
- **THEN** the column header shows the pt-BR label for "To-do"

### Requirement: Drag and drop
Users with `gentasks.task.write` SHALL be able to drag a card within its column or to another column, and users with `gentasks.board.manage` SHALL be able to drag columns to reorder them. The board updates as soon as the item is dropped, sends the move request, and on failure restores the previous order and shows an error. Keyboard users SHALL be able to perform the same moves.

#### Scenario: Move fails
- **WHEN** the move request returns an error
- **THEN** the card returns to its original column and position and a localized error is shown

### Requirement: Task editing
Creating a task SHALL ask for a title, with optional description, assignee, labels, and urgency. Opening a card SHALL show a task panel where these fields can be edited, and only the changed fields are sent. From the panel, a user can archive the task (with `gentasks.task.write`) or delete it after confirming (with `gentasks.task.delete`).

#### Scenario: Edit only urgency
- **WHEN** a user changes only the urgency in the panel
- **THEN** the `PATCH` body contains only `urgency`

### Requirement: Columns and labels management
Users with `gentasks.board.manage` SHALL be able to add, rename, and delete columns, and to create, rename, recolor, and delete labels. If the API refuses a deletion with `column_not_empty` or `last_column`, the UI SHALL show a specific localized message.

#### Scenario: Delete a column that still has tasks
- **WHEN** the API responds `column_not_empty`
- **THEN** the UI explains that the column's tasks must be moved, archived, or deleted first

### Requirement: Archived tasks and filters
The board page SHALL offer a view of archived tasks with a restore action, and filters by urgency, label, and assignee that narrow the visible cards on the client without changing their stored order.

#### Scenario: Filter by urgency
- **WHEN** the user filters by `urgent`
- **THEN** only urgent cards are visible, in their stored order

### Requirement: Localized copy and theme colors
All gentasks copy SHALL come from en and pt-BR translation files (rule 36). Urgency and label colors SHALL come from theme variables (rule 81).

#### Scenario: No hardcoded colors
- **WHEN** label chips render
- **THEN** their colors come from CSS theme variables named after the palette value
