## Purpose

Makes the web app follow the installation's enabled modules at runtime, so module UI appears only when its module is enabled (rule 58).

## ADDED Requirements

### Requirement: Load capabilities after sign-in
After the session is restored or a user signs in, the web app SHALL fetch `GET /api/core/v1/capabilities` and keep the enabled module ids for the rest of the session. If the request fails, the app SHALL treat every module as disabled and keep core screens working.

#### Scenario: Gentasks enabled
- **WHEN** capabilities list `gentasks`
- **THEN** the app treats gentasks as enabled

#### Scenario: Capabilities request fails
- **WHEN** the capabilities request returns an error
- **THEN** no module UI is shown and core pages still work

### Requirement: Module UI hidden when disabled
Navigation entries and routes of a module SHALL be shown only while that module is enabled. Opening a module route while it is disabled SHALL show a "module unavailable" message, not an error.

#### Scenario: Sidebar entry
- **WHEN** gentasks is not enabled
- **THEN** the sidebar has no Tasks entry

#### Scenario: Direct URL to a disabled module
- **WHEN** a user opens `/tasks` while gentasks is not enabled
- **THEN** the page shows the localized "module unavailable" message
