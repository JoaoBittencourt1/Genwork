## Purpose

Lets each board decide which column a task moves to for each GitHub pull request event, and which branches count as final for merges.

## ADDED Requirements

### Requirement: Read board GitHub settings
`GET /api/gentasks/v1/boards/{token}/github-settings` (permission `gentasks.board.read`) SHALL return `openedColumnToken`, `reviewRequestedColumnToken`, and `mergedColumnToken` (each a column token or `null`, where `null` means "do not move") and `finalBranches` (a list of branch names). A board whose settings were never saved SHALL read as: opened `null`, review requested `null`, merged the board's `done` preset column (or `null` when the board has none), final branches `main` and `master`.

#### Scenario: Never configured board
- **WHEN** a board with the default columns has never saved GitHub settings
- **THEN** gentasks responds `200` with opened `null`, review requested `null`, merged the `done` column's token, and final branches `["main", "master"]`

#### Scenario: Deleted column
- **WHEN** a configured column is deleted afterwards
- **THEN** the corresponding field reads as `null`

#### Scenario: Unknown board
- **WHEN** the board token does not exist or the board is deleted
- **THEN** gentasks responds `404`

### Requirement: Update board GitHub settings
`PATCH /api/gentasks/v1/boards/{token}/github-settings` (permission `gentasks.board.manage`) SHALL follow rule 17: an omitted field keeps its stored value, a column field set to `null` clears it, and `finalBranches` set to `null` is rejected. Column tokens SHALL belong to the same board and not be deleted. `finalBranches` SHALL hold 1 to 20 distinct branch names, each 1 to 255 characters with no whitespace; names are compared case-sensitively and duplicates are removed. The response is the full settings, as in the read endpoint.

#### Scenario: Single-field update
- **WHEN** a board manager patches only `reviewRequestedColumnToken`
- **THEN** that column is stored and every other field keeps its previous value, including defaults never saved before

#### Scenario: Stop moving on an event
- **WHEN** `mergedColumnToken` is sent as `null`
- **THEN** the merged mapping is cleared and merges no longer move tasks on this board

#### Scenario: Column from another board
- **WHEN** a column token belongs to a different board or is deleted
- **THEN** gentasks responds `400` with a validation error on that field

#### Scenario: Empty or null final branches
- **WHEN** `finalBranches` is `[]` or `null`
- **THEN** gentasks responds `400` with a validation error on `finalBranches`

#### Scenario: Missing permission
- **WHEN** a user without `gentasks.board.manage` patches the settings
- **THEN** gentasks responds `403`

### Requirement: Settings independent of the connection
Board GitHub settings SHALL be readable and editable whether or not a GitHub App is connected, and SHALL survive disconnecting and reconnecting.

#### Scenario: Configure before connecting
- **WHEN** a board manager saves settings while GitHub is disconnected
- **THEN** the settings are stored and apply once an App is connected
