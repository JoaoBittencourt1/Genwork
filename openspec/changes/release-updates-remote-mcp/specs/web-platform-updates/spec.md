## Purpose

Lets administrators check for and install platform updates from the web app's home page, and follow the update until the new version is running.

## ADDED Requirements

### Requirement: Update card on the home page
The home page SHALL show an update card only to users holding `core.system.update`. The card SHALL show the installed version and the time of the last check. When the installation cannot be updated (`updatable: false`), the card SHALL explain why and offer no update action.

#### Scenario: Admin opens the home page
- **WHEN** an admin opens the home page
- **THEN** the update card shows the installed version and a "Check for updates" button

#### Scenario: Member opens the home page
- **WHEN** a member without `core.system.update` opens the home page
- **THEN** no update card is shown and the update status is never requested

#### Scenario: Development checkout
- **WHEN** the status reports `updatable: false` with reason `development_installation`
- **THEN** the card says updates are installed with git in development checkouts and shows no update button

### Requirement: Checking for updates from the card
Pressing "Check for updates" SHALL request a check and show a busy state until it answers. The card SHALL then show either that the installation is up to date, or the newer version with a link to its release notes and an "Update to v<version>" button. A failed check SHALL show the translated error and keep the button available.

#### Scenario: Newer version found
- **WHEN** the check answers with `latest.version` `0.5.2` and the installed version is `0.5.0`
- **THEN** the card shows "Update to v0.5.2" and a link to the release notes

#### Scenario: Up to date
- **WHEN** the check answers with no newer release
- **THEN** the card says the installation runs the latest version

### Requirement: Starting an update
"Update to v<version>" SHALL ask for confirmation. The confirmation SHALL say that Genwork will be unavailable for a few minutes, that a database backup is taken first, and that a failed update returns to the current version. Confirming SHALL request the update for exactly that version.

#### Scenario: Admin confirms
- **WHEN** the admin confirms the update to `0.5.2`
- **THEN** the web app sends `{"version": "0.5.2"}` and the card switches to the progress view

#### Scenario: Version changed meanwhile
- **WHEN** the update request answers `409 version_mismatch`
- **THEN** the card explains that a newer release appeared and checks again

### Requirement: Following an update
While the status is `updating`, the card SHALL show the current step and poll the status every few seconds. Network errors and `502`/`503` answers during the update SHALL count as "restarting" and SHALL NOT be shown as failures. When the status is `succeeded` with the target version installed, the web app SHALL reload so the browser runs the new version. When the status is `failed`, the card SHALL show the failed step and whether the previous version was restored. Opening the home page during an update SHALL show the progress view.

#### Scenario: Core restarts during the update
- **WHEN** the status request fails with a network error while an update runs
- **THEN** the card shows "Restarting Genwork…" and keeps polling

#### Scenario: Update finished
- **WHEN** polling returns `state: "succeeded"` and `installedVersion` equal to the target
- **THEN** the web app reloads the page

#### Scenario: Update failed and rolled back
- **WHEN** polling returns `state: "failed"` with `failure.rolledBack: true`
- **THEN** the card says the update failed at the reported step and that Genwork is back on the previous version

#### Scenario: Polling never comes back
- **WHEN** the status cannot be read for 15 minutes in a row during an update
- **THEN** the card stops polling and tells the admin to check the server with `genwork update --check`

### Requirement: Localized copy
Every text of the update card SHALL come from translation keys in pt-BR and English, including step names and error codes.

#### Scenario: Portuguese interface
- **WHEN** the interface language is pt-BR
- **THEN** the card, the confirmation, the steps, and the errors are shown in Portuguese
