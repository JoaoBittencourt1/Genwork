## Purpose

Gives administrators a page to connect GitHub in a few clicks and gives board managers a place to map GitHub pull request events to columns.

## ADDED Requirements

### Requirement: GitHub integration page
The web app SHALL offer a GitHub integration page under the gentasks area, reachable from the boards page by users holding `gentasks.github.manage`. It SHALL show the connection state and, when connected, the App name, owner, a link to choose repositories on GitHub, a link to the App's settings, and a disconnect action that asks for confirmation. All copy SHALL be localized (pt-BR and English).

#### Scenario: Entry point visibility
- **WHEN** a user without `gentasks.github.manage` opens the boards page
- **THEN** no link to the integration page is shown

#### Scenario: Connected state
- **WHEN** an admin opens the page while an App is connected
- **THEN** it shows the App name and owner, a "choose repositories" link to the install URL, and a disconnect button

#### Scenario: Unreadable credentials
- **WHEN** the connection state is `unreadable`
- **THEN** the page explains that the credentials can no longer be read and offers disconnecting so the admin can connect again

#### Scenario: Module disabled
- **WHEN** gentasks is not enabled
- **THEN** the integration routes are unavailable like every other gentasks route

### Requirement: Connect in a few clicks
From the disconnected state, the page SHALL let the admin optionally type a GitHub organization and press "Connect GitHub"; the app then requests the manifest and submits it to GitHub with a form post, without the admin copying any value. The callback route SHALL send GitHub's `code` and `state` to the API, show the outcome, and lead to installing the App on repositories.

#### Scenario: Connect to a personal account
- **WHEN** an admin leaves the organization empty and presses "Connect GitHub"
- **THEN** the browser lands on GitHub's App creation page with the manifest pre-filled

#### Scenario: Callback success
- **WHEN** GitHub redirects back with a code and state that the API accepts
- **THEN** the page shows the App as connected and offers "choose repositories"

#### Scenario: Callback failure
- **WHEN** the API rejects the code or state, or GitHub redirects back without a code
- **THEN** the page shows a localized error and lets the admin start again

### Requirement: Public URL warning
The integration page SHALL warn, before and after connecting, when the webhook URL cannot be reached by GitHub: its scheme is not `https`, or its host is `localhost`, a loopback address, or a private network address.

#### Scenario: Local installation
- **WHEN** the webhook URL is `http://localhost:8485/api/gentasks/v1/github/webhook`
- **THEN** the page warns that GitHub cannot deliver events to this installation

#### Scenario: Public installation
- **WHEN** the webhook URL is `https://genwork.acme.com/api/gentasks/v1/github/webhook`
- **THEN** no warning is shown

### Requirement: Board GitHub settings section
The board settings page, which only users holding `gentasks.board.manage` can open, SHALL include a GitHub section where they pick, for each event (PR opened, review requested, merged), a column of the board or "do not move", and edit the list of final branches. When GitHub is not connected, the section SHALL say so and, for users holding `gentasks.github.manage`, link to the integration page.

#### Scenario: Pick columns
- **WHEN** a board manager picks `In review` for "review requested" and saves
- **THEN** only that field is sent to the API and the section shows the saved value

#### Scenario: Edit final branches
- **WHEN** a board manager adds `staging` to the final branches
- **THEN** the list saved is the previous one plus `staging`

#### Scenario: Invalid branch list
- **WHEN** the board manager removes every final branch
- **THEN** saving is blocked with a localized message before any request is sent

#### Scenario: Not connected
- **WHEN** GitHub is disconnected
- **THEN** the section shows that GitHub is not connected, keeps the settings editable, and links admins to the integration page
