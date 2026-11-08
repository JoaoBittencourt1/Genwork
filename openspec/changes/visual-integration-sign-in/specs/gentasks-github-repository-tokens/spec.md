## Purpose

Issues short-lived GitHub installation tokens from the gentasks GitHub App for the repositories bots work in.

## ADDED Requirements

### Requirement: Issue a repository token
A bot member holding `gentasks.repository.write` SHALL obtain a token with `POST /api/gentasks/v1/github/repository-tokens` for 1 to 10 repositories. Each repository SHALL be configured on a board or linked to a task's pull request. The token SHALL be limited to those repositories with `contents: write`, `pull_requests: write`, and `metadata: read`. It SHALL be returned with its expiry and never stored.

#### Scenario: Board repository
- **WHEN** a bot asks for a token for the repository of its board
- **THEN** it receives a token and its expiry

#### Scenario: A person asks
- **WHEN** an admin, not a bot, asks for a token with the permission
- **THEN** the request fails with 403 `repository_tokens_for_bots_only`

#### Scenario: Unknown repository
- **WHEN** a caller asks for a repository no board or task uses
- **THEN** the request fails with 400 on `repositories`

#### Scenario: App not installed
- **WHEN** the App is not installed on the repository
- **THEN** the request fails with 409 `github_app_not_installed`

#### Scenario: Missing permissions
- **WHEN** GitHub refuses the requested permissions
- **THEN** the request fails with 409 `github_app_permissions_missing`

### Requirement: App permissions
A GitHub App created through the manifest flow SHALL request `contents: write`, `pull_requests: write`, and `metadata: read`.

#### Scenario: New App
- **WHEN** an admin creates the App from Genwork
- **THEN** the manifest asks for those permissions
