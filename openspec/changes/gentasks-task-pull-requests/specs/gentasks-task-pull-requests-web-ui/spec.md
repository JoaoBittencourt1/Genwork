## ADDED Requirements

### Requirement: Pull request chip on the task card
A task card SHALL show, when the task has linked pull requests, a chip with a pull request icon and the localized count ("1 PR", "4 PRs" / "1 PR", "4 PRs" in pt-BR). Activating the chip SHALL open a menu listing the pull requests and SHALL NOT open the task dialog or start a drag. A task without pull requests SHALL show no chip.

#### Scenario: Count
- **WHEN** a task has four linked pull requests
- **THEN** its card shows a chip reading "4 PRs"

#### Scenario: Open the list
- **WHEN** someone clicks the chip
- **THEN** a menu lists the four pull requests, the task dialog stays closed, and the card does not move

#### Scenario: Keyboard
- **WHEN** someone focuses the chip with Tab and presses Enter
- **THEN** the menu opens and its items can be chosen with the arrow keys and Enter

#### Scenario: No pull requests
- **WHEN** a task has no linked pull request
- **THEN** its card shows no chip

### Requirement: Pull request list items
Each listed pull request SHALL show an icon for its state (open, draft, merged, closed, or unknown), its title — or `owner/repo#number` when the title is unknown — the `owner/repo#number` reference, and the author's avatar when known. Colors come only from theme variables (rule 81). Choosing an item SHALL open the pull request's `url` in a new tab with `rel="noopener noreferrer"`. The list SHALL be ordered open, draft, unknown, merged, closed, and by descending number within a state.

#### Scenario: Navigate to GitHub
- **WHEN** someone chooses `acme/api#12` in the list
- **THEN** a new tab opens on the pull request URL returned by the API

#### Scenario: Unknown title
- **WHEN** a pull request's title is `null`
- **THEN** the item shows `acme/api#3` as its title with the unknown-state icon

#### Scenario: Order
- **WHEN** a task has a closed `#31`, a merged `#32`, an open `#34`, and an open `#35`
- **THEN** the list shows `#35`, `#34`, `#32`, `#31`

### Requirement: Pull requests in the task dialog
The task dialog's properties panel SHALL show a "Pull requests" section listing the task's linked pull requests with the same items, order, and navigation as the card menu, for everyone who can read the board. The section SHALL be hidden when the task has none or when the dialog is creating a task.

#### Scenario: Dialog list
- **WHEN** someone opens a task with two linked pull requests
- **THEN** the properties panel lists both, and choosing one opens it in a new tab without closing the dialog

#### Scenario: No pull requests
- **WHEN** someone opens a task without linked pull requests
- **THEN** the dialog shows no pull requests section
