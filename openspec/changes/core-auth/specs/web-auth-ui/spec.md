## ADDED Requirements

### Requirement: Same-origin API access
The web app SHALL call core only through relative paths (`/api/...`, `/.well-known/...`) and SHALL contain no API base URL. In development, the Vite dev server SHALL proxy those paths to core, using a proxy target from the web app's environment and failing to start when it is missing.

#### Scenario: Development proxy
- **WHEN** a developer runs the web app with `DEV_API_PROXY_TARGET` set
- **THEN** requests from the browser to `/api/...` and `/.well-known/...` reach core through the dev server

#### Scenario: Missing proxy target
- **WHEN** the dev server starts without `DEV_API_PROXY_TARGET`
- **THEN** it exits with an error naming the missing variable

### Requirement: Sign in
The web app SHALL provide a login screen with e-mail and password that signs in with `client: "web"`, shows the instance name from the discovery document, and maps error codes to localized messages. After signing in, the user SHALL land on the home screen.

#### Scenario: Correct credentials
- **WHEN** a user submits valid credentials
- **THEN** they land on the home screen, which greets them by name and offers sign-out

#### Scenario: Wrong credentials
- **WHEN** the API responds `invalid_credentials`
- **THEN** the screen shows one localized message for wrong e-mail or password, without saying which

### Requirement: Token handling
The web app SHALL keep the access token only in memory, never in `localStorage`, `sessionStorage`, or a readable cookie. When a request returns `401 invalid_token`, it SHALL call refresh once and retry; concurrent requests SHALL share a single refresh. On page load it SHALL try a refresh to restore the session.

#### Scenario: Page reload keeps the user signed in
- **WHEN** a signed-in user reloads the page within the session lifetime
- **THEN** the app restores the session through refresh and does not show the login screen

#### Scenario: Refresh fails
- **WHEN** refresh responds `401`
- **THEN** the app clears in-memory state and shows the login screen

#### Scenario: Concurrent expiry
- **WHEN** three requests fail with `invalid_token` at the same time
- **THEN** the app performs exactly one refresh call and retries the three requests

### Requirement: Accept an invite
The route `/invite/:secret` SHALL load the invite, show the instance name and the invited e-mail as read-only, ask for name, password, and password confirmation, and on success sign the user in and land on the home screen. Invalid links SHALL show a localized "invite not found or expired" screen.

#### Scenario: Valid invite
- **WHEN** a person opens a valid invite link and submits name and matching valid passwords
- **THEN** their account is created and they land on the home screen signed in

#### Scenario: Password confirmation mismatch
- **WHEN** the two password fields differ
- **THEN** the form shows a localized error and does not call the API

### Requirement: Manage invites
Users whose `/me` permissions include `core.invite.read` SHALL see an "Invites" screen listing invites with status; with `core.invite.create` they SHALL be able to invite (e-mail + role), see and copy the link, and resend; with `core.invite.revoke` they SHALL be able to revoke. Controls for missing permissions SHALL be hidden, and the navigation entry SHALL be hidden without `core.invite.read`.

#### Scenario: Admin invites someone
- **WHEN** an admin submits the invite form
- **THEN** the new invite appears as pending and its link is shown with a copy button, along with whether the e-mail was sent

#### Scenario: Member without permissions
- **WHEN** a `member` signs in
- **THEN** the Invites entry is not shown, and opening its route directly shows a localized "no permission" screen

### Requirement: Localization
All user-visible copy in the web app SHALL come from translation files for `pt-BR` and `en`, selected from the browser language with `pt-BR` as the fallback. Every API error code used in this change SHALL have a translation.

#### Scenario: Portuguese browser
- **WHEN** the browser language is `pt-BR`
- **THEN** every screen in this change is shown in Portuguese
