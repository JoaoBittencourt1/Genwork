## Purpose

Lets an administrator connect a gentasks installation to GitHub by creating a private GitHub App through GitHub's App manifest flow, and keeps that App's credentials safe at rest.

## ADDED Requirements

### Requirement: Connection status
`GET /api/gentasks/v1/github/connection` (permission `gentasks.board.read`) SHALL return the connection `state` — `disconnected`, `connected`, or `unreadable` — and the `webhookUrl` GitHub delivers events to (always present, so clients can warn before connecting) and, when an App is stored, its `appSlug`, `ownerLogin`, `appUrl` (the App's settings page on GitHub), and `installUrl` (where repositories are picked). The response SHALL never contain the webhook secret or the private key.

#### Scenario: Nothing connected
- **WHEN** no GitHub App has been connected
- **THEN** gentasks responds `200` with `state` `disconnected`, the webhook URL, and no App fields

#### Scenario: Connected App
- **WHEN** an App is stored and its credentials decrypt with the current key
- **THEN** gentasks responds `200` with `state` `connected`, the App's slug, owner login, App URL, install URL, and webhook URL, and no secret material

#### Scenario: Encryption key changed
- **WHEN** an App is stored but its credentials no longer decrypt with the current encryption key
- **THEN** gentasks responds `200` with `state` `unreadable`

### Requirement: Start the manifest flow
`POST /api/gentasks/v1/github/connection/manifest` (permission `gentasks.github.manage`) SHALL accept an optional `organization` (a GitHub organization login) and return the `formAction` URL, the `manifest` JSON, and a `state` value the client posts to GitHub. The manifest SHALL describe a private App with: a suggested name unique to the installation (the admin can change it on GitHub), `url` and `hook_attributes.url` derived from `GENWORK_PUBLIC_URL` (the webhook at `/api/gentasks/v1/github/webhook`), `redirect_url` and `setup_url` pointing at the web app's GitHub integration routes, permissions `pull_requests: read` and `metadata: read`, and the `pull_request` event. `formAction` SHALL target the organization's App creation page when `organization` is given and the caller's personal one otherwise. The `state` SHALL be bound to the calling user and expire after one hour.

#### Scenario: Personal account
- **WHEN** an admin starts the flow without `organization`
- **THEN** gentasks responds `200` with `formAction` on `{GITHUB_WEB_URL}/settings/apps/new` and a manifest whose webhook URL is `{GENWORK_PUBLIC_URL}/api/gentasks/v1/github/webhook`

#### Scenario: Organization
- **WHEN** an admin starts the flow with `organization` `acme`
- **THEN** `formAction` is `{GITHUB_WEB_URL}/organizations/acme/settings/apps/new`

#### Scenario: Invalid organization login
- **WHEN** `organization` contains characters GitHub logins cannot have, or is longer than 39 characters
- **THEN** gentasks responds `400` with a validation error on `organization`

#### Scenario: Already connected
- **WHEN** an App is already stored
- **THEN** gentasks responds `409`

#### Scenario: Missing permission
- **WHEN** a user without `gentasks.github.manage` starts the flow
- **THEN** gentasks responds `403`

### Requirement: Complete the manifest flow
`POST /api/gentasks/v1/github/connection` (permission `gentasks.github.manage`) SHALL accept the `code` and `state` GitHub returned, verify the `state`, exchange the code at `{GITHUB_API_URL}/app-manifests/{code}/conversions`, and store the App's id, slug, owner login, App URL, webhook secret, and private key. The OAuth client secret GitHub also returns SHALL be discarded. On success it responds `201` with the connection status.

#### Scenario: Successful connection
- **WHEN** an admin posts a valid code with the state they started
- **THEN** gentasks stores the App, responds `201` with `state` `connected`, and the stored webhook secret and private key are encrypted

#### Scenario: Expired or foreign state
- **WHEN** the state expired, was tampered with, or was issued to another user
- **THEN** gentasks responds `400` with a validation error on `state` and calls nothing on GitHub

#### Scenario: Code rejected by GitHub
- **WHEN** GitHub answers the conversion with `404` or `422`
- **THEN** gentasks responds `400` with a validation error on `code` and stores nothing

#### Scenario: GitHub unreachable
- **WHEN** the conversion request fails or GitHub answers `5xx`
- **THEN** gentasks responds `502` and stores nothing

#### Scenario: Already connected
- **WHEN** an App is already stored
- **THEN** gentasks responds `409` without calling GitHub

### Requirement: Disconnect
`DELETE /api/gentasks/v1/github/connection` (permission `gentasks.github.manage`) SHALL remove the stored App and its credentials and respond `204`. Board GitHub settings SHALL be kept, so reconnecting resumes with them. Removing the App on GitHub stays the admin's action on GitHub.

#### Scenario: Disconnect a connected App
- **WHEN** an admin disconnects
- **THEN** gentasks responds `204`, the status reads `disconnected`, the credentials are gone from the database, and webhooks signed with the old secret are rejected

#### Scenario: Nothing to disconnect
- **WHEN** no App is stored
- **THEN** gentasks responds `204`

### Requirement: Credentials at rest
The webhook secret and the private key SHALL be stored only encrypted with an authenticated cipher keyed from `GENTASKS_ENCRYPTION_KEY`, and SHALL never appear in API responses, logs, or error messages. gentasks SHALL refuse to start when `GENTASKS_ENCRYPTION_KEY`, `GITHUB_WEB_URL`, or `GITHUB_API_URL` is missing or invalid.

#### Scenario: Missing encryption key
- **WHEN** gentasks starts without `GENTASKS_ENCRYPTION_KEY`
- **THEN** it exits with an error naming the missing variable

#### Scenario: Ciphertext tampered with
- **WHEN** a stored credential's ciphertext is altered
- **THEN** decryption fails and the connection reads as `unreadable` instead of using corrupted data
