## ADDED Requirements

### Requirement: Server data refreshes automatically
The web app SHALL re-read the data of every visible screen every 10 seconds while the browser tab is visible, and immediately when the tab becomes visible or focused again. It SHALL NOT poll while the tab is hidden. The interval SHALL be defined once in the web app.

#### Scenario: Another person moves a task
- **WHEN** two people have the same board open and one moves a task to another column
- **THEN** the other person's board shows the task in the new column within 10 seconds, without reloading the page

#### Scenario: GitHub automation
- **WHEN** a merged pull request moves a task while someone has its board open
- **THEN** the board shows the task in the merged column within 10 seconds

#### Scenario: Other screens
- **WHEN** an admin has the members page open and another admin changes a member's role
- **THEN** the members page shows the new role within 10 seconds

#### Scenario: Hidden tab
- **WHEN** the tab with a board is in the background
- **THEN** no board request is made until the tab is visible again, and then the board is read immediately

### Requirement: Screens that must not poll
The invitation acceptance screen, the AI assistant authorization screen, and the instance discovery data SHALL NOT refresh automatically. Screens with their own polling cadence SHALL keep it.

#### Scenario: Invitation
- **WHEN** someone keeps the accept-invitation page open for a minute
- **THEN** the invitation is inspected only once

#### Scenario: Own cadence
- **WHEN** the genmonitor host page is open
- **THEN** it keeps refreshing at its own interval

### Requirement: Refresh failures keep the data on screen
A failed automatic refresh SHALL keep showing the last data loaded and SHALL NOT replace the screen with an error. An error SHALL be shown only when a screen's first load fails. A later successful refresh SHALL show the new data.

#### Scenario: Network blip
- **WHEN** a board refresh fails after the board loaded
- **THEN** the board stays on screen with no error, and the next successful refresh updates it

#### Scenario: First load fails
- **WHEN** a board cannot be loaded at all
- **THEN** the error is shown, as today

### Requirement: Refresh never undoes a change being saved
While a task move, column move, or reassignment the person made on the board is being saved, the board SHALL NOT refresh automatically; it SHALL resume once the save finishes and the board is re-read.

#### Scenario: Move in flight
- **WHEN** someone drops a task in another column and the save takes 3 seconds while a refresh is due
- **THEN** the task stays in the new column the whole time and the board is re-read after the save

### Requirement: Refresh never overwrites a draft
Automatic refresh SHALL NOT change what a person is editing in the task dialog. The board settings form and the board GitHub settings form SHALL keep their draft while the data behind them refreshes, unless someone else saved that same form in the meantime: then the form SHALL reload with the saved values, so saving a stale draft never silently reverts the other person's change.

#### Scenario: Editing a task
- **WHEN** someone is typing a task description and the board refreshes with another person's change to the same task's title
- **THEN** the dialog keeps the typed description and title draft, and saving sends only the fields the person changed

#### Scenario: Settings unchanged elsewhere
- **WHEN** someone is editing the board name and the board refreshes because another person moved a task
- **THEN** the settings form keeps the typed name

#### Scenario: Same settings saved elsewhere
- **WHEN** someone is editing the board's GitHub settings and another person saves different GitHub settings for the same board
- **THEN** the form reloads with the other person's saved values
